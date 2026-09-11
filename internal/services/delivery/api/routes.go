package api

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/constants"
	"food-delivery-backend/internal/middleware"
	userrepository "food-delivery-backend/internal/services/users/repository/repository"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(protected *gin.RouterGroup, deps *app.Container) {
	if deps.DeliveryService == nil {
		return
	}
	h := NewHandler(deps.DeliveryService)
	protected.GET("/orders/:orderId/delivery", h.Get)
	approved := middleware.RequireApprovedOnboarding(userrepository.NewRepository(deps.DB, deps.Redis))
	driver := protected.Group("/driver", middleware.RequireRole(constants.RoleDriver), approved)
	driver.GET("/delivery", h.GetForDriver)
	driver.PATCH("/delivery/status", h.UpdateForDriver)
}
