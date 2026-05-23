package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const validatedBodyContextKey = "validated_body"

type ValidationDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationFunc func(sanitized map[string]any, c *gin.Context) (any, []ValidationDetail)

func RequestValidator(allowedFields []string, fn ValidationFunc) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedFields))
	for _, field := range allowedFields {
		allowed[strings.TrimSpace(field)] = struct{}{}
	}

	return func(c *gin.Context) {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"status":     "error",
				"error_code": "VALIDATION_ERROR",
				"message":    "invalid request body",
				"details":    []ValidationDetail{{Field: "body", Message: "failed to read request body"}},
			})
			return
		}
		if len(bodyBytes) == 0 {
			bodyBytes = []byte("{}")
		}

		parsed := map[string]any{}
		if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"status":     "error",
				"error_code": "VALIDATION_ERROR",
				"message":    "invalid JSON body",
				"details":    []ValidationDetail{{Field: "body", Message: "malformed JSON"}},
			})
			return
		}

		sanitized := map[string]any{}
		for key, value := range parsed {
			if _, ok := allowed[key]; ok {
				sanitized[key] = value
			}
		}

		payload, details := fn(sanitized, c)
		if len(details) > 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"status":     "error",
				"error_code": "VALIDATION_ERROR",
				"message":    "validation failed",
				"details":    details,
			})
			return
		}

		c.Set(validatedBodyContextKey, payload)
		c.Next()
	}
}

func GetValidatedBody[T any](c *gin.Context) (T, bool) {
	var zero T
	raw, ok := c.Get(validatedBodyContextKey)
	if !ok {
		return zero, false
	}
	val, ok := raw.(T)
	if !ok {
		return zero, false
	}
	return val, true
}
