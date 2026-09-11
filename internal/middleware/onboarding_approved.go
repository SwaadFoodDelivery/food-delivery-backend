package middleware

import (
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/users/repository/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequireApprovedOnboarding checks the current database user on every request.
// The completion flag also permits approved seed accounts without onboarding rows.
func RequireApprovedOnboarding(repo repository.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawUserID, hasUserID := c.Get(constants.AuthContextUserIDKey)
		rawRole, hasRole := c.Get(constants.AuthContextRoleKey)
		if !hasUserID || !hasRole {
			abortError(c, http.StatusUnauthorized, apperrors.CodeUnauthorized, "missing auth claims")
			return
		}
		userID, validUserID := rawUserID.(string)
		role, validRole := rawRole.(string)
		userID = strings.TrimSpace(userID)
		if !validUserID || uuid.Validate(userID) != nil || !validRole || strings.TrimSpace(role) == "" {
			abortError(c, http.StatusUnauthorized, apperrors.CodeUnauthorized, "invalid auth claims")
			return
		}
		if repo == nil {
			abortError(c, http.StatusInternalServerError, apperrors.CodeInternal, "repository not configured")
			return
		}

		user, err := repo.FindUserByID(c.Request.Context(), userID)
		if err != nil {
			if repository.IsNotFound(err) {
				abortError(c, http.StatusUnauthorized, apperrors.CodeUserNotFound, "user not found")
				return
			}
			abortError(c, http.StatusInternalServerError, apperrors.CodeInternal, "failed to validate onboarding approval")
			return
		}
		if user == nil {
			abortError(c, http.StatusUnauthorized, apperrors.CodeUserNotFound, "user not found")
			return
		}
		if user.Role != role {
			abortError(c, http.StatusForbidden, apperrors.CodeForbidden, "role mismatch")
			return
		}
		if user.AccountStatus != constants.AccountStatusActive {
			abortError(c, http.StatusForbidden, apperrors.CodeAccountNotActive, "account is not active")
			return
		}
		if !user.OnboardingComplete {
			abortError(c, http.StatusForbidden, apperrors.CodeForbidden, "onboarding approval required")
			return
		}

		c.Next()
	}
}
