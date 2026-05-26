package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func StructuredLogger(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Header("X-Request-ID", requestID)

		reqLog := log.With().
			Str("request_id", requestID).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Str("client_ip", c.ClientIP()).
			Logger()
		c.Request = c.Request.WithContext(reqLog.WithContext(c.Request.Context()))

		start := time.Now()
		c.Next()

		userID, _ := c.Get(ContextUserIDKey)
		event := reqLog.Info()
		status := c.Writer.Status()
		if status >= 500 {
			event = reqLog.Error()
		} else if status >= 400 {
			event = reqLog.Warn()
		}
		event.
			Interface("user_id", userID).
			Int("status", status).
			Dur("latency", time.Since(start)).
			Msg("http_request")
	}
}
