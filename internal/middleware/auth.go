package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	appredis "food-delivery-backend/infra/redis"
	"food-delivery-backend/pkg/config"
	apputils "food-delivery-backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	rds "github.com/redis/go-redis/v9"
)

const (
	ContextUserIDKey    = "user_id"
	ContextRoleKey      = "role"
	ContextSessionIDKey = "sid"
	ContextClaimsKey    = "auth_claims"
	sessionTTL          = 24 * time.Hour
)

func JWTAuthMiddleware(cfg *config.Config, rc *rds.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(auth, "Bearer ") {
			abortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid bearer")
			return
		}
		rawToken := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if rawToken == "" || cfg.JWT.Secret == "" {
			abortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
			return
		}

		claims, err := apputils.ParseAccessToken(cfg.JWT.Secret, rawToken)
		if err != nil {
			switch {
			case errors.Is(err, jwt.ErrTokenExpired):
				abortError(c, http.StatusUnauthorized, "TOKEN_EXPIRED", "token expired")
			default:
				abortError(c, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token")
			}
			return
		}
		if claims.ID == "" {
			abortError(c, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token claims")
			return
		}

		sKey := appredis.SessionKey(claims.ID)
		sessionData, err := rc.HGetAll(c.Request.Context(), sKey).Result()
		if err != nil || len(sessionData) == 0 {
			abortError(c, http.StatusUnauthorized, "SESSION_NOT_FOUND", "session not found")
			return
		}
		if strings.ToLower(strings.TrimSpace(sessionData["is_active"])) != "true" {
			abortError(c, http.StatusUnauthorized, "SESSION_REVOKED", "session revoked")
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextRoleKey, claims.Role)
		c.Set(ContextSessionIDKey, claims.ID)
		c.Set(ContextClaimsKey, claims)

		_ = rc.Expire(c.Request.Context(), sKey, sessionTTL).Err()
		c.Next()
	}
}

func abortError(c *gin.Context, statusCode int, code, message string) {
	c.AbortWithStatusJSON(statusCode, gin.H{
		"status":     "error",
		"error_code": code,
		"message":    message,
		"details":    []string{},
	})
}
