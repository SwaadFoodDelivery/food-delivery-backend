package routes

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/services/users/business"
	"food-delivery-backend/internal/services/users/repository/repository"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(v1Public *gin.RouterGroup, v1Protected *gin.RouterGroup, deps *app.Container) {
	repo := repository.NewRepository(deps.DB, deps.Redis)
	svc := business.NewService(repo, deps.Config, deps.Logger, deps.OTPProvider, deps.StorageProvider)

	RegisterAuthRoutes(v1Public, v1Protected, deps, svc)
	RegisterOnboardingRoutes(v1Public, v1Protected, deps, svc)
}
