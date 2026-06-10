package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/users/repository/repository"

	"github.com/gin-gonic/gin"
)

func RequireOnboardingAccess(repo repository.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if repo == nil {
			abortError(c, http.StatusInternalServerError, apperrors.CodeInternal, "repository not configured")
			return
		}

		rawUserID, ok := c.Get(constants.AuthContextUserIDKey)
		if !ok {
			abortError(c, http.StatusUnauthorized, apperrors.CodeUnauthorized, "missing auth claims")
			return
		}
		userID := strings.TrimSpace(castToString(rawUserID))
		if userID == "" {
			abortError(c, http.StatusUnauthorized, apperrors.CodeUnauthorized, "invalid auth claims")
			return
		}

		user, err := repo.FindUserByID(c.Request.Context(), userID)
		if err != nil {
			if repository.IsNotFound(err) {
				abortError(c, http.StatusUnauthorized, apperrors.CodeUserNotFound, "user not found")
				return
			}
			abortError(c, http.StatusInternalServerError, apperrors.CodeInternal, "failed to validate onboarding access")
			return
		}

		if claimRole, ok := c.Get(constants.AuthContextRoleKey); ok && strings.TrimSpace(castToString(claimRole)) != strings.TrimSpace(user.Role) {
			abortError(c, http.StatusForbidden, apperrors.CodeForbidden, "role mismatch")
			return
		}
		if strings.ToLower(strings.TrimSpace(user.AccountStatus)) != constants.AccountStatusActive {
			abortError(c, http.StatusForbidden, apperrors.CodeAccountNotActive, "account is not active")
			return
		}
		if user.OnboardingComplete {
			abortError(c, http.StatusConflict, apperrors.CodeOnboardingAlreadyCompleted, "onboarding already completed")
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
