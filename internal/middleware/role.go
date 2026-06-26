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
			abortError(c, http.StatusUnauthorized, apperrors.CodeUnauthorized, "missing auth claims")
			return
		}
		rs, _ := role.(string)
		if !allow[rs] {
			abortError(c, http.StatusForbidden, apperrors.CodeForbidden, "insufficient role")
			return
		}
		c.Next()
	}
}
