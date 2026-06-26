package routes

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/common/api/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(v1Public *gin.RouterGroup, v1Protected *gin.RouterGroup, deps *app.Container) {
	sessionHandler := handler.NewSessionHandler(deps.Config)
	initSession := v1Public.Group("/auth",
		middleware.APIKeyMiddleware(deps.Config.ClientAPIKey),
		middleware.LeakyBucketRateLimit(deps.Redis, "init_session_ip", 30.0/60.0, 10, 60, middleware.IPKeyFunc),
	)
	initSession.POST("/init-session", sessionHandler.InitSession)

	uploadsHandler := handler.NewUploadsHandler(deps)
	uploads := v1Protected.Group("/uploads")
	presigned := uploads.Group("/presigned-url",
		middleware.LeakyBucketRateLimit(deps.Redis, "uploads_presigned_url", 30.0/60.0, 20, 60, middleware.UserIDKeyFunc),
	)
	presigned.POST("", uploadsHandler.PresignURL)
}
