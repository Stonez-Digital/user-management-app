package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// This exercises Configure's real Gin middleware and in-memory limiter. It does
// not open a database connection, call an external service, or use school accounts.
func TestAuthRateLimitHTTPEnforcementAndWindowReset(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, time.November, 4, 15, 0, 0, 0, time.UTC)
	router := gin.New()
	configureWithClock(router, func() time.Time { return now })
	router.POST("/auth/probe", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	doRequest := func(clientIP string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/auth/probe", nil)
		// Simulate the same Cloudflare-to-Render socket peer for every caller;
		// only CF-Connecting-IP changes between independent clients.
		// Without an authenticated proxy secret, the socket peer is the only
		// trusted client address; forwarded headers must not split buckets.
		req.RemoteAddr = clientIP + ":443"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}

	for request := 1; request <= authRateLimit; request++ {
		response := doRequest("198.51.100.10")
		if response.Code != http.StatusNoContent {
			t.Fatalf("request %d: expected %d, got %d: %s", request, http.StatusNoContent, response.Code, response.Body.String())
		}
	}
	if got := doRequest("198.51.100.10"); got.Code != http.StatusTooManyRequests {
		t.Fatalf("11th request: expected HTTP 429, got %d: %s", got.Code, got.Body.String())
	} else {
		if got.Header().Get("X-RateLimit-Limit") != "10" {
			t.Errorf("expected X-RateLimit-Limit=10, got %q", got.Header().Get("X-RateLimit-Limit"))
		}
		if got.Header().Get("Retry-After") != "60" {
			t.Errorf("expected Retry-After=60, got %q", got.Header().Get("Retry-After"))
		}
	}

	// A different visitor IP must receive its own independent allowance.
	if got := doRequest("198.51.100.11"); got.Code != http.StatusNoContent {
		t.Fatalf("independent client: expected HTTP 204, got %d: %s", got.Code, got.Body.String())
	}

	// Advance the injected clock, not wall time, to verify the real middleware
	// resets the original client's bucket after one complete minute.
	now = now.Add(authRateWindow)
	if got := doRequest("198.51.100.10"); got.Code != http.StatusNoContent {
		t.Fatalf("after one-minute window: expected HTTP 204, got %d: %s", got.Code, got.Body.String())
	}
}


// The public forgot-password endpoint is covered by the same auth-wide
// per-client limiter as login, refresh, and logout.
func TestForgotPasswordEndpointRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("API_PROXY_HMAC_SECRET", "")
	t.Setenv("REDIS_URL", "")
	t.Setenv("APP_ENV", "")

	now := time.Date(2026, time.November, 16, 12, 0, 0, 0, time.UTC)
	router := gin.New()
	configureWithClock(router, func() time.Time { return now })
	handlerCalls := 0
	router.POST("/auth/forgot-password", func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusOK, gin.H{"message": "if the account exists, a reset link will be sent"})
	})

	doRequest := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(`{"email":"person@example.test"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "198.51.100.25:443"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	for i := 1; i <= authRateLimit; i++ {
		response := doRequest()
		if response.Code != http.StatusOK {
			t.Fatalf("forgot-password request %d: expected HTTP 200, got %d: %s", i, response.Code, response.Body.String())
		}
	}
	limited := doRequest()
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("11th forgot-password request: expected HTTP 429, got %d: %s", limited.Code, limited.Body.String())
	}
	if got := limited.Header().Get("X-RateLimit-Limit"); got != "10" {
		t.Fatalf("expected X-RateLimit-Limit=10, got %q", got)
	}
	if got := limited.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("expected Retry-After=60, got %q", got)
	}
	if handlerCalls != authRateLimit {
		t.Fatalf("rate-limited request reached forgot-password handler: calls=%d, want %d", handlerCalls, authRateLimit)
	}
}
