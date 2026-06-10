package middleware

import (
	"net/http"
	"time"

	"food-delivery-backend/internal/constants"

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
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": constants.ResponseStatusSuccess, "data": v})
			return
		}
		c.Next()
		_ = rc.Set(ctx, rkey, "ok", 24*time.Hour).Err()
	}
}
