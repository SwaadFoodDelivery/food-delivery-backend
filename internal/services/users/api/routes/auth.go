package routes

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/users/api/handler"
	"food-delivery-backend/internal/services/users/business"
	"food-delivery-backend/internal/services/users/validations"

	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(v1Public *gin.RouterGroup, v1Protected *gin.RouterGroup, deps *app.Container, svc business.AuthService) {
	h := handler.NewAuthHandler(svc)

	publicAuth := v1Public.Group("/auth",
		middleware.GuestTokenMiddleware(deps.Config.JWT.GuestTokenSecret),
	)
	checkPhone := publicAuth.Group("/check-phone",
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_check_phone_ip", 30.0/60.0, 10, 60, middleware.IPKeyFunc),
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_check_phone_phone", 10.0/60.0, 5, 60, middleware.PhoneKeyFunc),
		middleware.RequestValidator([]string{"phone", "role"}, validations.ValidateCheckPhoneBody),
	)
	checkPhone.POST("", h.CheckPhone)

	register := publicAuth.Group("/register",
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_register_ip", 10.0/3600.0, 5, 3600, middleware.IPKeyFunc),
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_register", 18.0/3600.0, 6, 3600, middleware.PhoneRoleKeyFunc),
		middleware.RequestValidator([]string{"phone", "name", "email", "referral_code", "role"}, validations.ValidateRegisterBody),
	)
	register.POST("", h.Register)

	sendOTP := publicAuth.Group("/send-otp",
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_send_otp_ip", 20.0/3600.0, 5, 3600, middleware.IPKeyFunc),
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_send_otp", 10.0/3600.0, 4, 3600, middleware.PhoneKeyFunc),
		middleware.RequestValidator([]string{"phone"}, validations.ValidateSendOTPBody),
	)
	sendOTP.POST("", h.SendOTP)

	verifyOTP := publicAuth.Group("/verify-otp",
		middleware.RequestValidator([]string{"phone", "otp"}, validations.ValidateVerifyOTPBody),
	)
	verifyOTP.POST("", h.VerifyOTP)

	protectedAuth := v1Protected.Group("/auth")
	sendEmailOTP := protectedAuth.Group("/send-email-otp",
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_send_email_otp", 5.0/3600.0, 3, 3600, middleware.UserIDKeyFunc),
	)
	sendEmailOTP.POST("", h.SendEmailOTP)

	verifyEmail := protectedAuth.Group("/verify-email",
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_verify_email", 30.0/3600.0, 10, 3600, middleware.UserIDKeyFunc),
		middleware.RequestValidator([]string{"otp"}, validations.ValidateVerifyEmailBody),
	)
	verifyEmail.POST("", h.VerifyEmail)

	logout := protectedAuth.Group("/logout",
		middleware.LeakyBucketRateLimit(deps.Redis, "auth_logout", 10.0/60.0, 5, 60, middleware.UserIDKeyFunc),
	)
	logout.POST("", h.Logout)
}
