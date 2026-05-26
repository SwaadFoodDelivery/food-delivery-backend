package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	rds "github.com/redis/go-redis/v9"
)

func IdempotencyMiddleware(rc *rds.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		rkey := "idempotency:" + key
		if v, _ := rc.Get(ctx, rkey).Result(); v != "" {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": "success", "data": v})
			return
		}
		c.Next()
		_ = rc.Set(ctx, rkey, "ok", 24*time.Hour).Err()
	}
}
