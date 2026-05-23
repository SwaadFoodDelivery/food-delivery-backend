package router

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	authapi "food-delivery-backend/internal/services/auth/api"

	"github.com/gin-gonic/gin"
)

func NewRouter(deps *app.Container) *gin.Engine {
	r := gin.New()
	r.Use(middleware.StructuredLogger(deps.Logger), middleware.PanicRecovery(deps.Logger), middleware.SlidingWindowRateLimit(deps.Redis, deps.Config.RateLimit.DefaultPerMin, deps.Config.RateWindow()))

	v1 := r.Group("/api/v1")
	public := v1.Group("")
	protected := v1.Group("")
	protected.Use(middleware.JWTAuthMiddleware(deps.Config, deps.Redis))

	public.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	protected.GET("/me", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	authapi.RegisterRoutes(public, protected, deps)
	return r
}
