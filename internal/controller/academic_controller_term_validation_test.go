package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTermRequestAcceptsCustomSchoolTermNames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/terms", func(c *gin.Context) {
		var request termRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid term"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"name": request.Name})
	})

	body := `{"name":"Michaelmas","start_date":"2026-09-01T00:00:00Z","end_date":"2026-12-20T00:00:00Z","status":"planned"}`
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/terms", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected custom term name to be accepted, got status %d: %s", response.Code, response.Body.String())
	}
}

func TestTermRequestRejectsInvalidStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/terms", func(c *gin.Context) {
		var request termRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid term"})
			return
		}
		c.Status(http.StatusOK)
	})

	body := `{"name":"Michaelmas","start_date":"2026-09-01T00:00:00Z","end_date":"2026-12-20T00:00:00Z","status":"unknown"}`
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/terms", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid status to be rejected, got status %d", response.Code)
	}
}
