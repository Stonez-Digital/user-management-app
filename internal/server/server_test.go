package server

import (
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

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "{\"status\":\"ok\"}" {
		t.Fatalf("unexpected health response: %s", rec.Body.String())
	}
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
