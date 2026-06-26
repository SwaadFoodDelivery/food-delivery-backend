package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// APIKeyMiddleware rejects requests whose X-API-Key header does not match the
// configured key. Uses constant-time comparison to prevent timing attacks.
// If the configured key is empty (e.g. local dev without the var set), the
// middleware passes through — set CLIENT_API_KEY in prod to enforce it.
func APIKeyMiddleware(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.Next()
			return
		}

		provided := strings.TrimSpace(c.GetHeader(constants.HeaderAPIKey))
		if subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
			response.AbortError(c, http.StatusUnauthorized, apperrors.CodeUnauthorized, "invalid or missing API key", []string{})
			return
		}

		c.Next()
	}
}
