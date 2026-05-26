package routes

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/users/api/handler"
	"food-delivery-backend/internal/services/users/business"
	"food-delivery-backend/internal/services/users/validations"

	"github.com/gin-gonic/gin"
)

func RegisterOnboardingRoutes(v1Public *gin.RouterGroup, v1Protected *gin.RouterGroup, deps *app.Container, svc business.OnboardingService) {
	h := handler.NewOnboardingHandler(svc)

	onboarding := v1Protected.Group("/onboarding")

	init := onboarding.Group("/role/init",
		middleware.LeakyBucketRateLimit(deps.Redis, "onboarding_init", 30.0/60.0, 12, 60, middleware.UserIDKeyFunc),
		middleware.RequestValidator([]string{"role", "country"}, validations.ValidateInitOnboardingBody),
	)
	init.POST("", h.Init)

	submit := onboarding.Group("/:id/submit",
		middleware.LeakyBucketRateLimit(deps.Redis, "onboarding_submit", 20.0/60.0, 10, 60, middleware.UserIDKeyFunc),
	)
	submit.PATCH("", h.Submit)

	resubmit := onboarding.Group("/:id/resubmit",
		middleware.LeakyBucketRateLimit(deps.Redis, "onboarding_resubmit", 20.0/60.0, 10, 60, middleware.UserIDKeyFunc),
	)
	resubmit.POST("", h.Resubmit)

	callback := v1Public.Group("/onboarding/documents/uploaded",
		middleware.RequestValidator([]string{"s3_key"}, validations.ValidateDocumentUploadedBody),
	)
	callback.POST("", h.MarkUploaded)
}
