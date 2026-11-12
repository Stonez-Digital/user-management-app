package server

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	proxyTimestampHeader = "X-Stonez-Proxy-Timestamp"
	proxyNonceHeader     = "X-Stonez-Proxy-Nonce"
	proxySignatureHeader = "X-Stonez-Proxy-Signature"
	proxyClockSkew        = 60 * time.Second
	proxyNonceTTL         = 2 * time.Minute
)

type sharedStore struct {
	address  string
	username string
	password string
	database string
	useTLS   bool
	timeout  time.Duration
}

func newSharedStore(rawURL string) (*sharedStore, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	if parsed.Scheme != "redis" && parsed.Scheme != "rediss" {
		return nil, fmt.Errorf("REDIS_URL must use redis:// or rediss://")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("REDIS_URL must include a host")
	}
	store := &sharedStore{address: parsed.Host, useTLS: parsed.Scheme == "rediss", timeout: 2 * time.Second}
	if parsed.User != nil {
		store.username = parsed.User.Username()
		store.password, _ = parsed.User.Password()
	}
	if database := strings.TrimPrefix(parsed.Path, "/"); database != "" {
		if _, err := strconv.Atoi(database); err != nil {
			return nil, fmt.Errorf("REDIS_URL database path must be numeric")
		}
		store.database = database
	}
	return store, nil
}

func (s *sharedStore) command(ctx context.Context, args ...string) (any, error) {
	dialer := net.Dialer{Timeout: s.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", s.address)
	if err != nil {
		return nil, fmt.Errorf("connect shared store: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(s.timeout))
	if s.useTLS {
		host, _, splitErr := net.SplitHostPort(s.address)
		if splitErr != nil { host = s.address }
		tlsConn := tls.Client(conn, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return nil, fmt.Errorf("shared store TLS handshake: %w", err)
		}
		conn = tlsConn
	}
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	write := func(parts ...string) error {
		if _, err := fmt.Fprintf(writer, "*%d\r\n", len(parts)); err != nil { return err }
		for _, part := range parts {
			if _, err := fmt.Fprintf(writer, "$%d\r\n%s\r\n", len(part), part); err != nil { return err }
		}
		return writer.Flush()
	}
	read := func() (any, error) { return readRESP(reader) }
	if s.password != "" {
		auth := []string{"AUTH", s.password}
		if s.username != "" { auth = []string{"AUTH", s.username, s.password} }
		if err := write(auth...); err != nil { return nil, err }
		if _, err := read(); err != nil { return nil, fmt.Errorf("shared store AUTH: %w", err) }
	}
	if s.database != "" && s.database != "0" {
		if err := write("SELECT", s.database); err != nil { return nil, err }
		if _, err := read(); err != nil { return nil, fmt.Errorf("shared store SELECT: %w", err) }
	}
	if err := write(args...); err != nil { return nil, err }
	return read()
}

func readRESP(reader *bufio.Reader) (any, error) {
	prefix, err := reader.ReadByte()
	if err != nil { return nil, err }
	line, err := reader.ReadString('\n')
	if err != nil { return nil, err }
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch prefix {
	case '+':
		return line, nil
	case '-':
		return nil, errors.New(line)
	case ':':
		return strconv.ParseInt(line, 10, 64)
	case '$':
		length, parseErr := strconv.Atoi(line)
		if parseErr != nil { return nil, parseErr }
		if length < 0 { return nil, nil }
		if length > 1<<20 { return nil, errors.New("shared store response too large") }
		value := make([]byte, length+2)
		if _, err := io.ReadFull(reader, value); err != nil { return nil, err }
		return string(value[:length]), nil
	case '*':
		count, parseErr := strconv.Atoi(line)
		if parseErr != nil { return nil, parseErr }
		if count < 0 { return nil, nil }
		if count > 16 { return nil, errors.New("shared store array too large") }
		values := make([]any, count)
		for i := range values {
			values[i], err = readRESP(reader)
			if err != nil { return nil, err }
		}
		return values, nil
	default:
		return nil, fmt.Errorf("unsupported shared store response type %q", prefix)
	}
}

func (s *sharedStore) allowRate(ctx context.Context, key string, limit int, window time.Duration) (bool, int, int, error) {
	const script = "local n=redis.call('INCR',KEYS[1]); if n==1 then redis.call('PEXPIRE',KEYS[1],ARGV[1]); end; local ttl=redis.call('PTTL',KEYS[1]); return {n,ttl}"
	result, err := s.command(ctx, "EVAL", script, "1", key, strconv.FormatInt(window.Milliseconds(), 10))
	if err != nil { return false, 0, 0, err }
	values, ok := result.([]any)
	if !ok || len(values) != 2 { return false, 0, 0, errors.New("invalid shared rate-limit response") }
	count, ok1 := values[0].(int64)
	ttl, ok2 := values[1].(int64)
	if !ok1 || !ok2 { return false, 0, 0, errors.New("invalid shared rate-limit counters") }
	remaining := limit - int(count)
	if remaining < 0 { remaining = 0 }
	retry := int((ttl + 999) / 1000)
	if count > int64(limit) { return false, 0, retry, nil }
	return true, remaining, retry, nil
}

func (s *sharedStore) claimNonce(ctx context.Context, nonce string) (bool, error) {
	result, err := s.command(ctx, "SET", "stonez:proxy:nonce:"+nonce, "1", "NX", "PX", strconv.FormatInt(proxyNonceTTL.Milliseconds(), 10))
	if err != nil { return false, err }
	if result == nil { return false, nil }
	_, ok := result.(string)
	return ok, nil
}

func proxySignature(secret, method, requestURI, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	canonical := strings.Join([]string{method, requestURI, timestamp, nonce, hex.EncodeToString(digest[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func proxyAuthMiddleware(secret string, store *sharedStore, now func() time.Time) gin.HandlerFunc {
	if strings.TrimSpace(secret) == "" { return func(c *gin.Context) { c.Next() } }
	if now == nil { now = time.Now }
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/healthz" {
			c.Next()
			return
		}
		timestamp := c.GetHeader(proxyTimestampHeader)
		nonce := c.GetHeader(proxyNonceHeader)
		signature := c.GetHeader(proxySignatureHeader)
		unixSeconds, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil || nonce == "" || len(nonce) < 16 || len(nonce) > 128 || signature == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized proxy request"})
			return
		}
		requestTime := time.Unix(unixSeconds, 0)
		delta := now().Sub(requestTime)
		if delta < -proxyClockSkew || delta > proxyClockSkew {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "expired proxy request"})
			return
		}
		body, readErr := io.ReadAll(c.Request.Body)
		if readErr != nil {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		c.Request.Body = io.NopCloser(strings.NewReader(string(body)))
		expected := proxySignature(secret, c.Request.Method, c.Request.URL.RequestURI(), timestamp, nonce, body)
		provided, decodeErr := base64.RawURLEncoding.DecodeString(signature)
		expectedBytes, _ := base64.RawURLEncoding.DecodeString(expected)
		if decodeErr != nil || !hmac.Equal(provided, expectedBytes) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized proxy request"})
			return
		}
		if store == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "proxy replay protection unavailable"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		claimed, claimErr := store.claimNonce(ctx, nonce)
		if claimErr != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "proxy replay protection unavailable"})
			return
		}
		if !claimed {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "replayed proxy request"})
			return
		}
		c.Next()
	}
}
