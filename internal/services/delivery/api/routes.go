package api

import (
	"food-delivery-backend/internal/app"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(protected *gin.RouterGroup, deps *app.Container) {
	if deps.DeliveryService == nil {
		return
	}
	h := NewHandler(deps.DeliveryService)
	protected.GET("/orders/:orderId/delivery", h.Get)
}
