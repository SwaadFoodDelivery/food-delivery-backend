package api

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/payment/business"
	"food-delivery-backend/internal/services/payment/repository"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(protected *gin.RouterGroup, deps *app.Container) {
	svc := business.NewService(repository.NewPostgresRepository(deps.DB), business.NewMockProvider(), deps.DeliveryService)
	h := NewHandler(svc)
	orders := protected.Group("/orders", middleware.LeakyBucketRateLimit(deps.Redis, "payments", 10.0/60.0, 10, 60, middleware.UserIDKeyFunc))
	orders.POST("/:orderId/payment", h.Pay)
}
