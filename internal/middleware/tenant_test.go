package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestRequireSchoolContextUsesAuthenticatedSchoolIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	schoolID := uuid.New()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(SchoolIDKey, schoolID.String())
		c.Next()
	})
	r.Use(RequireSchoolContext())
	r.GET("/", func(c *gin.Context) {
		got, ok := SchoolIDFromContext(c)
		if !ok || got != schoolID {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestRequireSchoolContextRejectsMissingSchoolIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(SchoolIDKey, "")
		c.Next()
	})
	r.Use(RequireSchoolContext())
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}
