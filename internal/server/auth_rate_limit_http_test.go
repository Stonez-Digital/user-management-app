package server

import (
	"net/http"
	"net/http/httptest"
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
