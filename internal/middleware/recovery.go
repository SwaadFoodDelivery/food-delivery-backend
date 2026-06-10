package middleware

import (
	"net/http"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func PanicRecovery(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				ctxLog := zerolog.Ctx(c.Request.Context())
				if ctxLog == nil || ctxLog.GetLevel() == zerolog.Disabled {
					ctxLog = &log
				}
				ctxLog.Error().Interface("panic", r).Msg("panic_recovered")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": constants.ResponseStatusError, "error_code": apperrors.CodeInternal, "message": "internal error", "details": []string{}})
			}
		}()
		c.Next()
	}
}
