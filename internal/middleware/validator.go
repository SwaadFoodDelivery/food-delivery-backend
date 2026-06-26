package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

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
			response.AbortError(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid request body", []ValidationDetail{{Field: "body", Message: "failed to read request body"}})
			return
		}
		if len(bodyBytes) == 0 {
			bodyBytes = []byte("{}")
		}

		parsed := map[string]any{}
		if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
			response.AbortError(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid JSON body", []ValidationDetail{{Field: "body", Message: "malformed JSON"}})
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
			response.AbortError(c, http.StatusBadRequest, apperrors.CodeValidation, "validation failed", details)
			return
		}

		c.Set(constants.DefaultValidatedBodyContext, payload)
		c.Next()
	}
}

func GetValidatedBody[T any](c *gin.Context) (T, bool) {
	var zero T
	raw, ok := c.Get(constants.DefaultValidatedBodyContext)
	if !ok {
		return zero, false
	}
	val, ok := raw.(T)
	if !ok {
		return zero, false
	}
	return val, true
}
