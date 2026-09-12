package api

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	cartbusiness "food-delivery-backend/internal/services/cart/business"
	"food-delivery-backend/internal/services/cart/repository"
	"food-delivery-backend/internal/services/order/business"
	orderrepository "food-delivery-backend/internal/services/order/repository"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(v1Protected *gin.RouterGroup, deps *app.Container) {
	carts := cartbusiness.NewService(repository.NewPostgresRepository(deps.DB), deps.Config.Cart.HMACSecret)
	svc := business.NewService(carts, orderrepository.NewPostgresRepository(deps.DB), deps.DeliveryService)
	h := NewHandler(svc)
	orders := v1Protected.Group("/orders", middleware.LeakyBucketRateLimit(deps.Redis, "orders", 10.0/60.0, 10, 60, middleware.UserIDKeyFunc))
	orders.POST("/quote", h.Quote)
	orders.POST("/serviceability", h.Serviceability)
	orders.POST("", h.Place)
	orders.GET("", h.List)
	if deps.Config.GRPC.OrderRequired && deps.OrderClient != nil {
		orders.GET("/:orderId", NewGRPCReadHandler(deps.OrderClient))
	}
	orders.GET("/:orderId/history", h.History)
	orders.PATCH("/:orderId/cancel", h.Cancel)
}
