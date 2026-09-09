package api

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/constants"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/operations/business"
	"food-delivery-backend/internal/services/operations/repository"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(protected *gin.RouterGroup, deps *app.Container) {
	h := NewHandler(business.NewService(repository.NewPostgresRepository(deps.DB)))
	operations := protected.Group("/operations", middleware.RequireRole(constants.RoleRestaurantManager))
	operations.GET("/overview", h.Overview)
	operations.PATCH("/orders/:orderId/cancel", h.CancelOrder)
}
