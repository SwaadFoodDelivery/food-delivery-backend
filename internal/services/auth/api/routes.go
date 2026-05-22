package api

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/auth/business"
	"food-delivery-backend/internal/services/auth/repository"
	"food-delivery-backend/internal/services/auth/validations"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(v1Public *gin.RouterGroup, v1Protected *gin.RouterGroup, deps *app.Container) {
	repo := repository.NewRepository(deps.DB, deps.Redis)
	svc := business.NewService(repo, deps.Config, deps.Logger)
	h := NewHandler(svc)

	publicAuth := v1Public.Group("/auth")
	checkPhone := publicAuth.Group("/check-phone",
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_check_phone", 30.0/60.0, 10, 60, middleware.IPKeyFunc),
		middleware.RequestValidator([]string{"phone", "role"}, validations.ValidateCheckPhoneBody),
	)
	checkPhone.POST("", h.CheckPhone)

	register := publicAuth.Group("/register",
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_register", 18.0/3600.0, 6, 3600, middleware.PhoneRoleKeyFunc),
		middleware.RequestValidator([]string{"phone", "name", "email", "referral_code", "role"}, validations.ValidateRegisterBody),
	)
	register.POST("", h.Register)

	sendOTP := publicAuth.Group("/send-otp",
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_send_otp", 10.0/3600.0, 4, 3600, middleware.PhoneKeyFunc),
		middleware.RequestValidator([]string{"phone"}, validations.ValidateSendOTPBody),
	)
	sendOTP.POST("", h.SendOTP)

	verifyOTP := publicAuth.Group("/verify-otp",
		middleware.RequestValidator([]string{"phone", "otp"}, validations.ValidateVerifyOTPBody),
	)
	verifyOTP.POST("", h.VerifyOTP)

	protectedAuth := v1Protected.Group("/auth")
	logout := protectedAuth.Group("/logout",
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_logout", 10.0/60.0, 5, 60, middleware.UserIDKeyFunc),
	)
	logout.POST("", h.Logout)
}
