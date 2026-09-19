package httpx

import "github.com/gin-gonic/gin"

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorResponse{Error: ErrorBody{Code: code, Message: message}})
}

func Validation(c *gin.Context, details interface{}) {
	c.AbortWithStatusJSON(400, ErrorResponse{Error: ErrorBody{
		Code: "validation_error", Message: "request validation failed", Details: details,
	}})
}
