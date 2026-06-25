package middleware

import (
	"net/http"

	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/pkg/response"

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
				response.AbortError(c, http.StatusInternalServerError, apperrors.CodeInternal, "internal error", []string{})
			}
		}()
		c.Next()
	}
}
