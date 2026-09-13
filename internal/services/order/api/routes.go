package api

import (
	"net/http"

	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	cartbusiness "food-delivery-backend/internal/services/cart/business"
	"food-delivery-backend/internal/services/cart/repository"
	"food-delivery-backend/internal/services/order/business"
	orderrepository "food-delivery-backend/internal/services/order/repository"
	"food-delivery-backend/pkg/response"

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
	if deps.Config.GRPC.OrderRequired {
		if deps.OrderClient == nil {
			// Startup normally rejects this state. Keep route selection fail-closed
			// for alternate composition paths rather than silently using local SQL.
			unavailable := func(c *gin.Context) {
				response.Error(c, http.StatusServiceUnavailable, "ORDER_SERVICE_UNAVAILABLE", "order service unavailable", []string{})
			}
			orders.GET("", unavailable)
			orders.GET("/:orderId", unavailable)
		} else {
			orders.GET("", NewGRPCListHandler(deps.OrderClient))
			orders.GET("/:orderId", NewGRPCReadHandler(deps.OrderClient))
		}
	} else {
		orders.GET("", h.List)
	}
	orders.GET("/:orderId/history", h.History)
	orders.PATCH("/:orderId/cancel", h.Cancel)
}
