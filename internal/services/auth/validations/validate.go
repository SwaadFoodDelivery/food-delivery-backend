package validations

import (
	"net/mail"
	"strings"

	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/auth/models"
	"food-delivery-backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

func ValidateCheckPhoneBody(input map[string]any, _ *gin.Context) (any, []middleware.ValidationDetail) {
	req := models.CheckPhoneRequest{}
	details := make([]middleware.ValidationDetail, 0)

	phone := getString(input, "phone")
	if !utils.ValidateIndianPhone(phone) {
		details = append(details, middleware.ValidationDetail{Field: "phone", Message: "must be a valid phone number"})
	} else {
		req.Phone = utils.NormalizeIndianPhone(phone)
	}

	role := strings.TrimSpace(getString(input, "role"))
	if !validRole(role) {
		details = append(details, middleware.ValidationDetail{Field: "role", Message: "must be one of client, restaurant_owner, restaurant_manager, driver"})
	} else {
		req.Role = role
	}

	return req, details
}

func ValidateRegisterBody(input map[string]any, _ *gin.Context) (any, []middleware.ValidationDetail) {
	req := models.RegisterRequest{}
	details := make([]middleware.ValidationDetail, 0)

	phone := getString(input, "phone")
	if !utils.ValidateIndianPhone(phone) {
		details = append(details, middleware.ValidationDetail{Field: "phone", Message: "must be a valid phone number"})
	} else {
		req.Phone = utils.NormalizeIndianPhone(phone)
	}

	name := strings.TrimSpace(getString(input, "name"))
	if name == "" || len(name) > 100 {
		details = append(details, middleware.ValidationDetail{Field: "name", Message: "must be non-empty and max 100 chars"})
	} else {
		req.Name = name
	}

	email := strings.TrimSpace(getString(input, "email"))
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			details = append(details, middleware.ValidationDetail{Field: "email", Message: "must be a valid email"})
		} else {
			req.Email = email
		}
	}

	role := strings.TrimSpace(getString(input, "role"))
	if !validRole(role) {
		details = append(details, middleware.ValidationDetail{Field: "role", Message: "must be one of client, restaurant_owner, restaurant_manager, driver"})
	} else {
		req.Role = role
	}

	req.ReferralCode = strings.TrimSpace(getString(input, "referral_code"))
	return req, details
}

func ValidateSendOTPBody(input map[string]any, _ *gin.Context) (any, []middleware.ValidationDetail) {
	req := models.SendOTPRequest{}
	details := make([]middleware.ValidationDetail, 0)

	phone := getString(input, "phone")
	if !utils.ValidateIndianPhone(phone) {
		details = append(details, middleware.ValidationDetail{Field: "phone", Message: "must be a valid phone number"})
	} else {
		req.Phone = utils.NormalizeIndianPhone(phone)
	}

	return req, details
}

func ValidateVerifyOTPBody(input map[string]any, c *gin.Context) (any, []middleware.ValidationDetail) {
	req := models.VerifyOTPRequest{}
	details := make([]middleware.ValidationDetail, 0)

	phone := getString(input, "phone")
	if !utils.ValidateIndianPhone(phone) {
		details = append(details, middleware.ValidationDetail{Field: "phone", Message: "must be a valid phone number"})
	} else {
		req.Phone = utils.NormalizeIndianPhone(phone)
	}

	otp := strings.TrimSpace(getString(input, "otp"))
	if !utils.ValidateOTP(otp) {
		details = append(details, middleware.ValidationDetail{Field: "otp", Message: "must be exactly 6 digits"})
	} else {
		req.OTP = otp
	}

	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" || len(deviceID) > 255 {
		details = append(details, middleware.ValidationDetail{Field: "device_id", Message: "header X-Device-ID is required and max length is 255"})
	}

	return req, details
}

func getString(in map[string]any, key string) string {
	v, ok := in[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func validRole(role string) bool {
	switch role {
	case "client", "restaurant_owner", "restaurant_manager", "driver":
		return true
	default:
		return false
	}
}
