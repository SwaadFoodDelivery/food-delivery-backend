package validations

import (
	"net/mail"
	"regexp"
	"strings"
	"time"

	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/users/models"
	"food-delivery-backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

var rePhone = regexp.MustCompile(`^\+?[1-9]\d{6,14}$`)
var rePincode = regexp.MustCompile(`^[A-Z0-9]{3,10}$`)

// optionalString reads an optional field without asserting its type. A bare
// v.(string) panics when the client sends {"name": 123}, which gin turns into a
// 500; callers use `present` to skip absent fields and `isString` to reject
// wrong-typed ones as a 400.
func optionalString(in map[string]any, key string) (val string, present, isString bool) {
	v, ok := in[key]
	if !ok {
		return "", false, false
	}
	s, ok := v.(string)
	if !ok {
		return "", true, false
	}
	return strings.TrimSpace(s), true, true
}

func mustBeString(field string) middleware.ValidationDetail {
	return middleware.ValidationDetail{Field: field, Message: "must be a string"}
}

// ValidateUpdateProfileBody is used by RequestValidator middleware.
func ValidateUpdateProfileBody(input map[string]any, _ *gin.Context) (any, []middleware.ValidationDetail) {
	req := models.UpdateProfileRequest{}
	details := make([]middleware.ValidationDetail, 0)

	if s, present, isStr := optionalString(input, "name"); present {
		if !isStr {
			details = append(details, mustBeString("name"))
		} else if s == "" || len(s) > 100 {
			details = append(details, middleware.ValidationDetail{Field: "name", Message: "must be 1–100 characters"})
		} else {
			req.Name = &s
		}
	}

	// Email is rejected rather than ignored: silently dropping it would report
	// success for a change that never happened.
	if _, ok := input["email"]; ok {
		details = append(details, middleware.ValidationDetail{
			Field:   "email",
			Message: "cannot be updated here; use POST /users/me/email/send-otp then POST /users/me/email/verify",
		})
	}

	if s, present, isStr := optionalString(input, "date_of_birth"); present {
		if !isStr {
			details = append(details, mustBeString("date_of_birth"))
		} else if _, err := time.Parse("2006-01-02", s); err != nil {
			details = append(details, middleware.ValidationDetail{Field: "date_of_birth", Message: "must be YYYY-MM-DD"})
		} else {
			req.DateOfBirth = &s
		}
	}

	if s, present, isStr := optionalString(input, "gender"); present {
		s = strings.ToLower(s)
		if !isStr {
			details = append(details, mustBeString("gender"))
		} else if s != "male" && s != "female" && s != "other" {
			details = append(details, middleware.ValidationDetail{Field: "gender", Message: "must be male, female, or other"})
		} else {
			req.Gender = &s
		}
	}

	if v, ok := input["is_available"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			details = append(details, middleware.ValidationDetail{Field: "is_available", Message: "must be a boolean"})
		} else {
			req.IsAvailable = &b
		}
	}

	if s, present, isStr := optionalString(input, "current_city"); present {
		if !isStr {
			details = append(details, mustBeString("current_city"))
		} else if len(s) > 80 {
			details = append(details, middleware.ValidationDetail{Field: "current_city", Message: "must be at most 80 characters"})
		} else {
			req.CurrentCity = &s
		}
	}

	return req, details
}

// ValidateSendEmailUpdateOTPBody is used by RequestValidator middleware.
func ValidateSendEmailUpdateOTPBody(input map[string]any, _ *gin.Context) (any, []middleware.ValidationDetail) {
	req := models.SendEmailUpdateOTPRequest{}
	details := make([]middleware.ValidationDetail, 0)

	email := getString(input, "email")
	if email == "" {
		details = append(details, middleware.ValidationDetail{Field: "email", Message: "is required"})
	} else if _, err := mail.ParseAddress(email); err != nil {
		details = append(details, middleware.ValidationDetail{Field: "email", Message: "must be a valid email address"})
	} else {
		req.Email = email
	}

	return req, details
}

// ValidateVerifyEmailUpdateBody is used by RequestValidator middleware.
func ValidateVerifyEmailUpdateBody(input map[string]any, _ *gin.Context) (any, []middleware.ValidationDetail) {
	req := models.VerifyEmailUpdateRequest{}
	details := make([]middleware.ValidationDetail, 0)

	email := getString(input, "email")
	if email == "" {
		details = append(details, middleware.ValidationDetail{Field: "email", Message: "is required"})
	} else if _, err := mail.ParseAddress(email); err != nil {
		details = append(details, middleware.ValidationDetail{Field: "email", Message: "must be a valid email address"})
	} else {
		req.Email = email
	}

	otpCode := getString(input, "otp")
	if !utils.ValidateOTP(otpCode) {
		details = append(details, middleware.ValidationDetail{Field: "otp", Message: "must be exactly 6 digits"})
	} else {
		req.OTP = otpCode
	}

	return req, details
}

// ValidateCreateAddressBody is used by RequestValidator middleware.
func ValidateCreateAddressBody(input map[string]any, _ *gin.Context) (any, []middleware.ValidationDetail) {
	req := models.CreateAddressRequest{}
	details := make([]middleware.ValidationDetail, 0)

	req.Line1 = strings.TrimSpace(getString(input, "line1"))
	if req.Line1 == "" || len(req.Line1) > 255 {
		details = append(details, middleware.ValidationDetail{Field: "line1", Message: "is required and must be at most 255 characters"})
	}

	req.City = strings.TrimSpace(getString(input, "city"))
	if req.City == "" || len(req.City) > 100 {
		details = append(details, middleware.ValidationDetail{Field: "city", Message: "is required and must be at most 100 characters"})
	}

	req.State = strings.TrimSpace(getString(input, "state"))
	if req.State == "" || len(req.State) > 100 {
		details = append(details, middleware.ValidationDetail{Field: "state", Message: "is required and must be at most 100 characters"})
	}

	pincode := strings.TrimSpace(strings.ToUpper(getString(input, "pincode")))
	if pincode == "" || !rePincode.MatchString(pincode) {
		details = append(details, middleware.ValidationDetail{Field: "pincode", Message: "is required and must be 3–10 alphanumeric characters"})
	} else {
		req.Pincode = pincode
	}

	req.Line2 = strings.TrimSpace(getString(input, "line2"))
	req.Area = strings.TrimSpace(getString(input, "area"))
	req.Label = strings.TrimSpace(getString(input, "label"))

	if cp := strings.TrimSpace(getString(input, "contact_phone")); cp != "" {
		if !rePhone.MatchString(cp) {
			details = append(details, middleware.ValidationDetail{Field: "contact_phone", Message: "must be a valid phone number"})
		} else {
			req.ContactPhone = cp
		}
	}
	req.ContactName = strings.TrimSpace(getString(input, "contact_name"))

	if v, ok := input["latitude"]; ok {
		if f, ok := toFloat64(v); ok {
			req.Latitude = &f
		}
	}
	if v, ok := input["longitude"]; ok {
		if f, ok := toFloat64(v); ok {
			req.Longitude = &f
		}
	}

	if v, ok := input["is_default"]; ok {
		if b, ok := v.(bool); ok {
			req.IsDefault = b
		}
	}

	return req, details
}

// ValidateUpdateAddressBody is used by RequestValidator middleware.
func ValidateUpdateAddressBody(input map[string]any, _ *gin.Context) (any, []middleware.ValidationDetail) {
	req := models.UpdateAddressRequest{}
	details := make([]middleware.ValidationDetail, 0)

	if s, present, isStr := optionalString(input, "line1"); present {
		if !isStr {
			details = append(details, mustBeString("line1"))
		} else if s == "" || len(s) > 255 {
			details = append(details, middleware.ValidationDetail{Field: "line1", Message: "must be 1–255 characters"})
		} else {
			req.Line1 = &s
		}
	}
	if s, present, isStr := optionalString(input, "city"); present {
		if !isStr {
			details = append(details, mustBeString("city"))
		} else if s == "" || len(s) > 100 {
			details = append(details, middleware.ValidationDetail{Field: "city", Message: "must be 1–100 characters"})
		} else {
			req.City = &s
		}
	}
	if s, present, isStr := optionalString(input, "state"); present {
		if !isStr {
			details = append(details, mustBeString("state"))
		} else if s == "" || len(s) > 100 {
			details = append(details, middleware.ValidationDetail{Field: "state", Message: "must be 1–100 characters"})
		} else {
			req.State = &s
		}
	}
	if s, present, isStr := optionalString(input, "pincode"); present {
		s = strings.ToUpper(s)
		if !isStr {
			details = append(details, mustBeString("pincode"))
		} else if !rePincode.MatchString(s) {
			details = append(details, middleware.ValidationDetail{Field: "pincode", Message: "must be 3–10 alphanumeric characters"})
		} else {
			req.Pincode = &s
		}
	}

	if s, present, isStr := optionalString(input, "line2"); present {
		if !isStr {
			details = append(details, mustBeString("line2"))
		} else {
			req.Line2 = &s
		}
	}
	if s, present, isStr := optionalString(input, "area"); present {
		if !isStr {
			details = append(details, mustBeString("area"))
		} else {
			req.Area = &s
		}
	}
	if s, present, isStr := optionalString(input, "label"); present {
		if !isStr {
			details = append(details, mustBeString("label"))
		} else {
			req.Label = &s
		}
	}
	if s, present, isStr := optionalString(input, "contact_name"); present {
		if !isStr {
			details = append(details, mustBeString("contact_name"))
		} else {
			req.ContactName = &s
		}
	}
	if cp, present, isStr := optionalString(input, "contact_phone"); present {
		if !isStr {
			details = append(details, mustBeString("contact_phone"))
		} else if cp != "" && !rePhone.MatchString(cp) {
			details = append(details, middleware.ValidationDetail{Field: "contact_phone", Message: "must be a valid phone number"})
		} else {
			req.ContactPhone = &cp
		}
	}
	if v, ok := input["is_default"]; ok {
		if b, ok := v.(bool); ok {
			req.IsDefault = &b
		}
	}
	if v, ok := input["latitude"]; ok {
		if f, ok := toFloat64(v); ok {
			req.Latitude = &f
		}
	}
	if v, ok := input["longitude"]; ok {
		if f, ok := toFloat64(v); ok {
			req.Longitude = &f
		}
	}

	return req, details
}

// ValidateAddressID validates a UUID-format address ID from path params.
func ValidateAddressID(id string) (string, string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", "address ID is required"
	}
	if len(id) != 36 {
		return "", "address ID must be a valid UUID"
	}
	return id, ""
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
