package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireRole(roles ...string) gin.HandlerFunc {
	allow := map[string]bool{}
	for _, r := range roles {
		allow[strings.TrimSpace(r)] = true
	}
	return func(c *gin.Context) {
		role, ok := c.Get(ContextRoleKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": "error", "error_code": "UNAUTHORIZED", "message": "missing auth claims", "details": []string{}})
			return
		}
		rs, _ := role.(string)
		if !allow[rs] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": "error", "error_code": "FORBIDDEN", "message": "insufficient role", "details": []string{}})
			return
		}
		c.Next()
	}
}
