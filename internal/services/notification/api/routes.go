package api

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/services/notification/business"
	"food-delivery-backend/internal/services/notification/repository"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(protected *gin.RouterGroup, deps *app.Container) {
	h := NewHandler(business.NewService(repository.NewPostgresRepository(deps.DB)))
	notifications := protected.Group("/notifications")
	notifications.GET("", h.List)
	notifications.PATCH("/:notificationId/read", h.MarkRead)
	notifications.POST("/read-all", h.MarkAllRead)
}
