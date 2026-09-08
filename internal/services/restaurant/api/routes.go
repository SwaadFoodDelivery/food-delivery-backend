package api

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/constants"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/restaurant/business"
	"food-delivery-backend/internal/services/restaurant/repository"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(v1Public *gin.RouterGroup, deps *app.Container) {
	h := NewHandler(business.NewService(repository.NewPostgresRepository(deps.DB)))
	restaurants := v1Public.Group("/restaurants", middleware.LeakyBucketRateLimit(deps.Redis, "restaurant_discovery", 60.0/60.0, 60, 60, middleware.IPKeyFunc))
	restaurants.GET("", h.List)
	restaurants.GET("/:restaurantId", h.Get)
	restaurants.GET("/:restaurantId/menu", h.Menu)
}

func RegisterOwnerRoutes(v1Protected *gin.RouterGroup, deps *app.Container) {
	h := newOwnerHandler(business.NewOwnerService(repository.NewPostgresRepository(deps.DB)))
	restaurants := v1Protected.Group("/restaurants/:restaurantId/menu", middleware.RequireRole(constants.RoleRestaurantOwner))
	restaurants.POST("/categories/:categoryId/items", h.CreateItem)
	restaurants.PUT("/items/:itemId", h.UpdateItem)
	restaurants.DELETE("/items/:itemId", h.DeleteItem)
}
