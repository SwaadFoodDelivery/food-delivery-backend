package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"food-delivery-backend/internal/services/users/repository/repository"

	"github.com/gin-gonic/gin"
)

func RequireOnboardingAccess(repo repository.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if repo == nil {
			abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "repository not configured")
			return
		}

		rawUserID, ok := c.Get(ContextUserIDKey)
		if !ok {
			abortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth claims")
			return
		}
		userID := strings.TrimSpace(castToString(rawUserID))
		if userID == "" {
			abortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid auth claims")
			return
		}

		user, err := repo.FindUserByID(c.Request.Context(), userID)
		if err != nil {
			if repository.IsNotFound(err) {
				abortError(c, http.StatusUnauthorized, "USER_NOT_FOUND", "user not found")
				return
			}
			abortError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to validate onboarding access")
			return
		}

		if claimRole, ok := c.Get(ContextRoleKey); ok && strings.TrimSpace(castToString(claimRole)) != strings.TrimSpace(user.Role) {
			abortError(c, http.StatusForbidden, "FORBIDDEN", "role mismatch")
			return
		}
		if strings.ToLower(strings.TrimSpace(user.AccountStatus)) != "active" {
			abortError(c, http.StatusForbidden, "ACCOUNT_NOT_ACTIVE", "account is not active")
			return
		}
		if user.OnboardingComplete {
			abortError(c, http.StatusConflict, "ONBOARDING_ALREADY_COMPLETED", "onboarding already completed")
			return
		}

		c.Next()
	}
}

func castToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
