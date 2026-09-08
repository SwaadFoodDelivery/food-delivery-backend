package api

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/constants"
	"food-delivery-backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(protected *gin.RouterGroup, deps *app.Container) {
	if deps.DeliveryService == nil {
		return
	}
	h := NewHandler(deps.DeliveryService)
	protected.GET("/orders/:orderId/delivery", h.Get)
	driver := protected.Group("/driver", middleware.RequireRole(constants.RoleDriver))
	driver.GET("/delivery", h.GetForDriver)
	driver.PATCH("/delivery/status", h.UpdateForDriver)
}
