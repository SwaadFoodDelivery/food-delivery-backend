package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func PanicRecovery(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("panic_recovered")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "error", "error_code": "INTERNAL_ERROR", "message": "internal error", "details": []string{}})
			}
		}()
		c.Next()
	}
}
