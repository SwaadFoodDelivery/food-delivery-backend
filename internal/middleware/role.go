package middleware

import (
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"

	"github.com/gin-gonic/gin"
)

func RequireRole(roles ...string) gin.HandlerFunc {
	allow := map[string]bool{}
	for _, r := range roles {
		allow[strings.TrimSpace(r)] = true
	}
	return func(c *gin.Context) {
		role, ok := c.Get(constants.AuthContextRoleKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": constants.ResponseStatusError, "error_code": apperrors.CodeUnauthorized, "message": "missing auth claims", "details": []string{}})
			return
		}
		rs, _ := role.(string)
		if !allow[rs] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": constants.ResponseStatusError, "error_code": apperrors.CodeForbidden, "message": "insufficient role", "details": []string{}})
			return
		}
		c.Next()
	}
}
