package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestRequireSchoolContextAcceptsAuthenticatedSchool(t *testing.T) {
	gin.SetMode(gin.TestMode)
	schoolID := uuid.New()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(SchoolIDKey, schoolID.String())
		c.Next()
	})
	router.Use(RequireSchoolContext(nil))
	router.GET("/", func(c *gin.Context) {
		id, ok := SchoolIDFromContext(c)
		if !ok || id != schoolID {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for authenticated school context, got %d", recorder.Code)
	}
}

func TestRequireSchoolContextRejectsMissingAuthenticatedSchool(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/", RequireSchoolContext(nil), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without authenticated school context, got %d", recorder.Code)
	}
}
