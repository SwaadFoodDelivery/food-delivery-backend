package response

import (
	"net/http"

	"food-delivery-backend/internal/constants"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	Data      any    `json:"data,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
	Details   any    `json:"details,omitempty"`
}

func Success(c *gin.Context, statusCode int, data any) {
	c.JSON(statusCode, APIResponse{
		Status: constants.ResponseStatusSuccess,
		Data:   data,
	})
}

func Error(c *gin.Context, statusCode int, code string, message string, details any) {
	if message == "" {
		message = http.StatusText(statusCode)
	}
	c.JSON(statusCode, APIResponse{
		Status:    constants.ResponseStatusError,
		ErrorCode: code,
		Message:   message,
		Details:   details,
	})
}

func ErrorWithData(c *gin.Context, statusCode int, code string, message string, data any, details any) {
	if message == "" {
		message = http.StatusText(statusCode)
	}
	c.JSON(statusCode, APIResponse{
		Status:    constants.ResponseStatusError,
		ErrorCode: code,
		Message:   message,
		Data:      data,
		Details:   details,
	})
}

func AbortError(c *gin.Context, statusCode int, code string, message string, details any) {
	Error(c, statusCode, code, message, details)
	c.Abort()
}

func AbortSuccess(c *gin.Context, statusCode int, data any) {
	Success(c, statusCode, data)
	c.Abort()
}
