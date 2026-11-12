package server

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func signedTestRequest(method, path, body, timestamp, nonce, secret string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(proxyTimestampHeader, timestamp)
	req.Header.Set(proxyNonceHeader, nonce)
	req.Header.Set(proxySignatureHeader, proxySignature(secret, method, req.URL.RequestURI(), timestamp, nonce, []byte(body)))
	return req
}

func TestProxyAuthRejectsMissingAndInvalidSignatures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	router := gin.New()
	router.Use(proxyAuthMiddleware("test-secret", nil, func() time.Time { return now }))
	router.POST("/auth/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	missing := httptest.NewRequest(http.MethodPost, "/auth/test", strings.NewReader("payload"))
	missingResult := httptest.NewRecorder()
	router.ServeHTTP(missingResult, missing)
	if missingResult.Code != http.StatusUnauthorized {
		t.Fatalf("missing signature: expected 401, got %d", missingResult.Code)
	}

	invalid := signedTestRequest(http.MethodPost, "/auth/test", "payload", "1794484800", "0123456789abcdef0123456789abcdef", "wrong-secret")
	invalidResult := httptest.NewRecorder()
	router.ServeHTTP(invalidResult, invalid)
	if invalidResult.Code != http.StatusUnauthorized {
		t.Fatalf("invalid signature: expected 401, got %d", invalidResult.Code)
	}
}

func TestProxyAuthRejectsExpiredAndTamperedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	router := gin.New()
	router.Use(proxyAuthMiddleware("test-secret", nil, func() time.Time { return now }))
	router.POST("/auth/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	expired := signedTestRequest(http.MethodPost, "/auth/test", "payload", "1794484739", "0123456789abcdef0123456789abcdef", "test-secret")
	expiredResult := httptest.NewRecorder()
	router.ServeHTTP(expiredResult, expired)
	if expiredResult.Code != http.StatusUnauthorized {
		t.Fatalf("expired signature: expected 401, got %d", expiredResult.Code)
	}

	tampered := signedTestRequest(http.MethodPost, "/auth/test", "payload", "1794484800", "1123456789abcdef0123456789abcdef", "test-secret")
	tampered.Body = http.NoBody
	tamperedResult := httptest.NewRecorder()
	router.ServeHTTP(tamperedResult, tampered)
	if tamperedResult.Code != http.StatusUnauthorized {
		t.Fatalf("tampered body: expected 401, got %d", tamperedResult.Code)
	}
}

func TestProxyAuthRequiresSharedReplayStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	router := gin.New()
	router.Use(proxyAuthMiddleware("test-secret", nil, func() time.Time { return now }))
	router.POST("/auth/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := signedTestRequest(http.MethodPost, "/auth/test", "payload", "1794484800", "2123456789abcdef0123456789abcdef", "test-secret")
	result := httptest.NewRecorder()
	router.ServeHTTP(result, req)
	if result.Code != http.StatusServiceUnavailable {
		t.Fatalf("valid signature without shared replay store: expected 503, got %d", result.Code)
	}
}

func TestValidateEnvironmentRequiresSharedStoreForSecurityModes(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("API_PROXY_AUTH_REQUIRED", "true")
	t.Setenv("API_PROXY_HMAC_SECRET", "")
	t.Setenv("REQUIRE_SHARED_RATE_LIMIT", "")
	t.Setenv("REDIS_URL", "")
	if err := ValidateEnvironment(); err == nil {
		t.Fatal("expected missing proxy secret to fail validation")
	}

	t.Setenv("API_PROXY_HMAC_SECRET", "secret")
	if err := ValidateEnvironment(); err == nil {
		t.Fatal("expected missing shared store to fail validation")
	}

	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	if err := ValidateEnvironment(); err != nil {
		t.Fatalf("valid security configuration rejected: %v", err)
	}
}

func TestNewSharedStoreRejectsUnsupportedURLs(t *testing.T) {
	if _, err := newSharedStore("https://example.com"); err == nil {
		t.Fatal("expected unsupported scheme to fail")
	}
	if _, err := newSharedStore("redis:///0"); err == nil {
		t.Fatal("expected missing host to fail")
	}
}

func TestReadRESPParsesSimpleIntegerBulkAndNull(t *testing.T) {
	cases := []struct {
		input string
		want  any
	}{
		{"+OK\r\n", "OK"},
		{":42\r\n", int64(42)},
		{"$3\r\nfoo\r\n", "foo"},
		{"$-1\r\n", nil},
	}
	for _, test := range cases {
		got, err := readRESP(bufio.NewReader(strings.NewReader(test.input)))
		if err != nil {
			t.Fatalf("readRESP(%q): %v", test.input, err)
		}
		if got != test.want {
			t.Errorf("readRESP(%q) = %#v, want %#v", test.input, got, test.want)
		}
	}
}

func TestSharedRateLimitKeyDoesNotExposeClientIP(t *testing.T) {
	key := sharedRateLimitKey("auth", "198.51.100.7")
	if strings.Contains(key, "198.51.100.7") {
		t.Fatal("rate-limit key must not expose the visitor IP")
	}
	if key != sharedRateLimitKey("auth", "198.51.100.7") {
		t.Fatal("rate-limit key should be deterministic")
	}
}
