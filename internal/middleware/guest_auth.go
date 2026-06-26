package middleware

import (
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/pkg/response"
	apputils "food-delivery-backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// GuestTokenMiddleware validates the X-Guest-Token header on public auth endpoints.
// It ensures the caller first obtained a token via POST /api/v1/auth/init-session,
// binding all auth attempts to a trackable device session.
func GuestTokenMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			c.Next()
			return
		}

		raw := strings.TrimSpace(c.GetHeader(constants.HeaderGuestToken))
		if raw == "" {
			response.AbortError(c, http.StatusUnauthorized, apperrors.CodeGuestTokenMissing, "X-Guest-Token header is required", []string{})
			return
		}

		claims, err := apputils.ParseGuestToken(secret, raw)
		if err != nil {
			response.AbortError(c, http.StatusUnauthorized, apperrors.CodeGuestTokenInvalid, "invalid or expired guest token", []string{})
			return
		}

		deviceID := strings.TrimSpace(c.GetHeader(constants.HeaderDeviceID))
		if deviceID != "" && claims.DeviceID != "" && claims.DeviceID != deviceID {
			response.AbortError(c, http.StatusUnauthorized, apperrors.CodeGuestTokenInvalid, "device mismatch", []string{})
			return
		}

		c.Set(constants.GuestTokenSessionIDKey, claims.ID)
		c.Set(constants.GuestTokenDeviceIDKey, claims.DeviceID)
		c.Next()
	}
}
