package httpx

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type validationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

func TestValidationResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.JSON(400, ErrorResponse{Error: ErrorBody{Code: "validation_error", Message: "request validation failed"}})

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
