package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConfigureHealthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Configure(r)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK { t.Fatalf("expected 200, got %d", rec.Code) }
	if rec.Body.String() != "{"status":"ok"}" { t.Fatalf("unexpected health response: %s", rec.Body.String()) }
}

func TestConfigureCORSAllowsConfiguredOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://school.example")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Configure(r)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://school.example")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://school.example" {
		t.Fatalf("expected configured CORS origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestConfigureSecurityHeaders(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Configure(r)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	checks := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options": "DENY",
		"Referrer-Policy": "no-referrer",
		"Permissions-Policy": "camera=(), microphone=(), geolocation=()",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	}
	for header, want := range checks {
		if got := rec.Header().Get(header); got != want { t.Fatalf("expected %s=%q, got %q", header, want, got) }
	}
}

func TestConfigureAuthRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Configure(r)
	r.POST("/auth/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for i := 0; i < authRateLimit; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/test", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent { t.Fatalf("request %d: expected 204, got %d", i+1, rec.Code) }
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/test", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests { t.Fatalf("expected 429 after rate limit, got %d", rec.Code) }
	if rec.Header().Get("Retry-After") != "60" { t.Fatalf("expected Retry-After=60, got %q", rec.Header().Get("Retry-After")) }
}

func TestConfigureRequestBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Configure(r)
	r.POST("/test", func(c *gin.Context) {
		body, err := c.GetRawData()
		if err != nil { c.Status(http.StatusRequestEntityTooLarge); return }
		c.Data(http.StatusOK, "text/plain", body)
	})
	body := make([]byte, maxRequestBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/test", &byteReader{data: body})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge { t.Fatalf("expected 413 for oversized request, got %d", rec.Code) }
}

type byteReader struct { data []byte }
func (r *byteReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 { return 0, io.EOF }
	n := copy(p, r.data); r.data = r.data[n:]; return n, nil
}
func (r *byteReader) Close() error { return nil }
