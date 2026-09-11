package api

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/cart/business"
	"food-delivery-backend/internal/services/cart/repository"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(v1Protected *gin.RouterGroup, deps *app.Container) {
	h := NewHandler(business.NewService(repository.NewPostgresRepository(deps.DB), deps.Config.Cart.HMACSecret))
	cart := v1Protected.Group("/cart", middleware.LeakyBucketRateLimit(deps.Redis, "cart", 30.0/60.0, 30, 60, middleware.UserIDKeyFunc))
	cart.POST("", h.AddItem)
	cart.GET("/:cartToken", h.Get)
	cart.DELETE("/:cartToken/items/:cartItemId", h.DeleteItem)
	cart.DELETE("/:cartToken", h.Clear)
}
