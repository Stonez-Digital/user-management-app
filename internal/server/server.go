package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	maxRequestBodyBytes = 1 << 20
	authRateLimit       = 10
	authRateWindow      = time.Minute
	maxRateLimitEntries = 10000
	cleanupEvery       = 64
)

type rateLimitEntry struct {
	count       int
	windowStart time.Time
}

type rateLimiter struct {
	mu       sync.Mutex
	entries  map[string]rateLimitEntry
	requests uint64
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{entries: make(map[string]rateLimitEntry)}
}

func (l *rateLimiter) allow(key string, now time.Time) (bool, int) {
	if key == "" {
		key = "unknown"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.requests++
	if l.requests%cleanupEvery == 0 {
		for client, entry := range l.entries {
			if now.Sub(entry.windowStart) >= authRateWindow {
				delete(l.entries, client)
			}
		}
	}

	entry, ok := l.entries[key]
	if !ok {
		// Fail closed for new keys rather than allowing unbounded memory growth.
		if len(l.entries) >= maxRateLimitEntries {
			return false, 0
		}
		l.entries[key] = rateLimitEntry{count: 1, windowStart: now}
		return true, authRateLimit - 1
	}
	if now.Sub(entry.windowStart) >= authRateWindow {
		l.entries[key] = rateLimitEntry{count: 1, windowStart: now}
		return true, authRateLimit - 1
	}
	if entry.count >= authRateLimit {
		return false, 0
	}
	entry.count++
	l.entries[key] = entry
	return true, authRateLimit - entry.count
}

// clientIPFromRequest uses forwarding headers only when the immediate peer is
// explicitly listed in TRUSTED_PROXY_CIDRS. Otherwise it uses the socket peer.
// X-Forwarded-For is walked from right to left, skipping trusted proxies.
func clientIPFromRequest(remoteAddr, forwardedFor string, trusted []*net.IPNet) string {
	peer := parseIP(remoteAddr)
	if peer == nil {
		return "unknown"
	}
	if !isTrustedProxy(peer, trusted) {
		return peer.String()
	}

	chain := strings.Split(forwardedFor, ",")
	current := peer
	for i := len(chain) - 1; i >= 0; i-- {
		candidate := net.ParseIP(strings.TrimSpace(chain[i]))
		if candidate == nil {
			continue
		}
		current = candidate
		if !isTrustedProxy(candidate, trusted) {
			return candidate.String()
		}
	}
	return current.String()
}

func parseIP(address string) net.IP {
	host, _, err := net.SplitHostPort(strings.TrimSpace(address))
	if err == nil {
		return net.ParseIP(host)
	}
	return net.ParseIP(strings.Trim(strings.TrimSpace(address), "[]"))
}

func isTrustedProxy(ip net.IP, trusted []*net.IPNet) bool {
	for _, network := range trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func trustedProxyNetworks() ([]*net.IPNet, error) {
	raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if raw == "" {
		return nil, nil
	}
	var networks []*net.IPNet
	for _, item := range strings.Split(raw, ",") {
		_, network, err := net.ParseCIDR(strings.TrimSpace(item))
		if err != nil {
			return nil, fmt.Errorf("invalid TRUSTED_PROXY_CIDRS entry %q: %w", item, err)
		}
		networks = append(networks, network)
	}
	return networks, nil
}

func ValidateEnvironment() error {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		if strings.TrimSpace(os.Getenv("DB_DRIVER")) != "postgres" {
			return fmt.Errorf("production requires DB_DRIVER=postgres")
		}
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			return fmt.Errorf("production requires DATABASE_URL")
		}
		if strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")) == "" {
			return fmt.Errorf("production requires CORS_ALLOWED_ORIGINS")
		}
	}
	_, err := trustedProxyNetworks()
	return err
}

func Configure(r *gin.Engine) {
	allowed := configuredOrigins()
	limiter := newRateLimiter()
	trusted, err := trustedProxyNetworks()
	if err != nil {
		// ValidateEnvironment is called before the server starts. Keep Configure
		// defensive for tests and non-standard callers.
		panic(err)
	}

	r.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")

		if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)

		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			if origin == "" || allowed[origin] {
				c.Status(http.StatusNoContent)
				c.Abort()
				return
			}
			c.Status(http.StatusForbidden)
			c.Abort()
			return
		}

		if strings.HasPrefix(c.Request.URL.Path, "/auth/") {
			key := clientIPFromRequest(c.Request.RemoteAddr, c.Request.Header.Get("X-Forwarded-For"), trusted)
			allowedRequest, remaining := limiter.allow(key, time.Now())
			c.Header("X-RateLimit-Limit", fmt.Sprint(authRateLimit))
			c.Header("X-RateLimit-Remaining", fmt.Sprint(remaining))
			if !allowedRequest {
				c.Header("Retry-After", "60")
				c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many authentication requests; try again later"})
				c.Abort()
				return
			}
		}

		c.Next()
	})

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

func configuredOrigins() map[string]bool {
	origins := map[string]bool{}
	for _, raw := range strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",") {
		if origin := strings.TrimSpace(raw); origin != "" {
			origins[origin] = true
		}
	}
	if len(origins) == 0 {
		origins["http://localhost:3000"] = true
	}
	return origins
}

func Start(r *gin.Engine, port string) error {
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-shutdownSignal():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

func shutdownSignal() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
		<-signalCh
		close(ch)
	}()
	return ch
}
