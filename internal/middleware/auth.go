package middleware

import (
	"errors"
	"net/http"
	"strings"

	appredis "food-delivery-backend/infra/redis"
	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/pkg/config"
	"food-delivery-backend/pkg/response"
	apputils "food-delivery-backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	rds "github.com/redis/go-redis/v9"
)

func JWTAuthMiddleware(cfg *config.Config, rc *rds.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := strings.TrimSpace(c.GetHeader(constants.HeaderAuthorization))
		if !strings.HasPrefix(auth, constants.BearerPrefix) {
			abortError(c, http.StatusUnauthorized, apperrors.CodeUnauthorized, "invalid bearer")
			return
		}
		rawToken := strings.TrimSpace(strings.TrimPrefix(auth, constants.BearerPrefix))
		if rawToken == "" || cfg.JWT.Secret == "" {
			abortError(c, http.StatusUnauthorized, apperrors.CodeUnauthorized, "invalid token")
			return
		}

		claims, err := apputils.ParseAccessToken(cfg.JWT.Secret, rawToken)
		if err != nil {
			switch {
			case errors.Is(err, jwt.ErrTokenExpired):
				abortError(c, http.StatusUnauthorized, apperrors.CodeTokenExpired, "token expired")
			default:
				abortError(c, http.StatusUnauthorized, apperrors.CodeInvalidToken, "invalid token")
			}
			return
		}
		if claims.ID == "" {
			abortError(c, http.StatusUnauthorized, apperrors.CodeInvalidToken, "invalid token claims")
			return
		}

		sKey := appredis.SessionKey(claims.ID)
		sessionData, err := rc.HGetAll(c.Request.Context(), sKey).Result()
		if err != nil || len(sessionData) == 0 {
			abortError(c, http.StatusUnauthorized, apperrors.CodeSessionNotFound, "session not found")
			return
		}
		if strings.ToLower(strings.TrimSpace(sessionData["is_active"])) != constants.RedisSessionActiveValue {
			abortError(c, http.StatusUnauthorized, apperrors.CodeSessionRevoked, "session revoked")
			return
		}

		c.Set(constants.AuthContextUserIDKey, claims.UserID)
		c.Set(constants.AuthContextRoleKey, claims.Role)
		c.Set(constants.AuthContextSessionIDKey, claims.ID)
		c.Set(constants.AuthContextClaimsKey, claims)

		_ = rc.Expire(c.Request.Context(), sKey, constants.AuthSessionTTL).Err()
		c.Next()
	}
}

func abortError(c *gin.Context, statusCode int, code, message string) {
	response.AbortError(c, statusCode, code, message, []string{})
}
