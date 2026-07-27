package routes

import (
	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/users/api/handler"
	"food-delivery-backend/internal/services/users/business"
	"food-delivery-backend/internal/services/users/validations"

	"github.com/gin-gonic/gin"
)

func RegisterProfileRoutes(_ *gin.RouterGroup, v1Protected *gin.RouterGroup, deps *app.Container, svc business.ProfileService) {
	h := handler.NewProfileHandler(svc)

	me := v1Protected.Group("/users/me")

	profileGet := me.Group("/profile",
		middleware.LeakyBucketRateLimit(deps.Redis, "profile_get", 60.0/60.0, 30, 60, middleware.UserIDKeyFunc),
	)
	profileGet.GET("", h.GetProfile)

	profileUpdate := me.Group("/profile",
		middleware.LeakyBucketRateLimit(deps.Redis, "profile_update", 10.0/60.0, 5, 60, middleware.UserIDKeyFunc),
		middleware.RequestValidator(
			[]string{"name", "email", "date_of_birth", "gender", "is_available", "current_city"},
			validations.ValidateUpdateProfileBody,
		),
	)
	profileUpdate.PUT("", h.UpdateProfile)

	// Email changes are their own flow: the OTP proves the user owns the new
	// address, and only the verify step writes it. Both are keyed by user ID —
	// a logged-in caller can always mint a fresh guest token, so a guest-scoped
	// limiter would be trivial to reset.
	emailSendOTP := me.Group("/email/send-otp",
		middleware.LeakyBucketRateLimit(deps.Redis, "profile_email_send_otp", 5.0/3600.0, 3, 3600, middleware.UserIDKeyFunc),
		middleware.RequestValidator([]string{"email"}, validations.ValidateSendEmailUpdateOTPBody),
	)
	emailSendOTP.POST("", h.SendEmailUpdateOTP)

	emailVerify := me.Group("/email/verify",
		middleware.LeakyBucketRateLimit(deps.Redis, "profile_email_verify", 30.0/3600.0, 10, 3600, middleware.UserIDKeyFunc),
		middleware.RequestValidator([]string{"email", "otp"}, validations.ValidateVerifyEmailUpdateBody),
	)
	emailVerify.POST("", h.VerifyEmailUpdate)

	addrList := me.Group("/addresses",
		middleware.LeakyBucketRateLimit(deps.Redis, "address_list", 60.0/60.0, 30, 60, middleware.UserIDKeyFunc),
	)
	addrList.GET("", h.GetAddresses)

	addrCreate := me.Group("/addresses",
		middleware.LeakyBucketRateLimit(deps.Redis, "address_create", 10.0/60.0, 5, 60, middleware.UserIDKeyFunc),
		middleware.RequestValidator(
			[]string{"label", "line1", "line2", "area", "city", "state", "pincode", "latitude", "longitude", "contact_name", "contact_phone", "is_default"},
			validations.ValidateCreateAddressBody,
		),
	)
	addrCreate.POST("", h.CreateAddress)

	addrUpdate := me.Group("/addresses/:addressId",
		middleware.LeakyBucketRateLimit(deps.Redis, "address_update", 10.0/60.0, 5, 60, middleware.UserIDKeyFunc),
		middleware.RequestValidator(
			[]string{"label", "line1", "line2", "area", "city", "state", "pincode", "latitude", "longitude", "contact_name", "contact_phone", "is_default"},
			validations.ValidateUpdateAddressBody,
		),
	)
	addrUpdate.PUT("", h.UpdateAddress)

	addrDelete := me.Group("/addresses/:addressId",
		middleware.LeakyBucketRateLimit(deps.Redis, "address_delete", 10.0/60.0, 5, 60, middleware.UserIDKeyFunc),
	)
	addrDelete.DELETE("", h.DeleteAddress)
}
