package router

import (
	"net/http"

	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	cartroutes "food-delivery-backend/internal/services/cart/api"
	commonroutes "food-delivery-backend/internal/services/common/api/routes"
	deliveryroutes "food-delivery-backend/internal/services/delivery/api"
	orderroutes "food-delivery-backend/internal/services/order/api"
	paymentroutes "food-delivery-backend/internal/services/payment/api"
	restaurantroutes "food-delivery-backend/internal/services/restaurant/api"
	usersroutes "food-delivery-backend/internal/services/users/api/routes"
	"food-delivery-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

func NewRouter(deps *app.Container) *gin.Engine {
	r := gin.New()
	r.Use(middleware.StructuredLogger(deps.Logger), middleware.PanicRecovery(deps.Logger), middleware.SlidingWindowRateLimit(deps.Redis, deps.Config.RateLimit.DefaultPerMin, deps.Config.RateWindow()))

	v1 := r.Group("/api/v1")
	public := v1.Group("")
	protected := v1.Group("")
	protected.Use(middleware.JWTAuthMiddleware(deps.Config, deps.Redis))

	public.GET("/health", func(c *gin.Context) { response.Success(c, http.StatusOK, gin.H{"healthy": true}) })
	protected.GET("/me", func(c *gin.Context) { response.Success(c, http.StatusOK, gin.H{"authenticated": true}) })

	commonroutes.RegisterRoutes(public, protected, deps)
	cartroutes.RegisterRoutes(protected, deps)
	orderroutes.RegisterRoutes(protected, deps)
	deliveryroutes.RegisterRoutes(protected, deps)
	paymentroutes.RegisterRoutes(protected, deps)
	restaurantroutes.RegisterRoutes(public, deps)
	restaurantroutes.RegisterOwnerRoutes(protected, deps)
	usersroutes.RegisterRoutes(public, protected, deps)
	return r
}
