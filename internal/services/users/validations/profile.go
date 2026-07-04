package validations

import (
	"net/mail"
	"regexp"
	"strings"
	"time"

	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/users/models"

	"github.com/gin-gonic/gin"
)

var rePhone = regexp.MustCompile(`^\+?[1-9]\d{6,14}$`)
var rePincode = regexp.MustCompile(`^[A-Z0-9]{3,10}$`)

// ValidateUpdateProfileBody is used by RequestValidator middleware.
func ValidateUpdateProfileBody(input map[string]any, _ *gin.Context) (any, []middleware.ValidationDetail) {
	req := models.UpdateProfileRequest{}
	details := make([]middleware.ValidationDetail, 0)

	if v, ok := input["name"]; ok {
		s := strings.TrimSpace(v.(string))
		if s == "" || len(s) > 100 {
			details = append(details, middleware.ValidationDetail{Field: "name", Message: "must be 1–100 characters"})
		} else {
			req.Name = &s
		}
	}

	if v, ok := input["email"]; ok {
		s := strings.TrimSpace(v.(string))
		if _, err := mail.ParseAddress(s); err != nil {
			details = append(details, middleware.ValidationDetail{Field: "email", Message: "must be a valid email address"})
		} else {
			req.Email = &s
		}
	}

	if v, ok := input["date_of_birth"]; ok {
		s := strings.TrimSpace(v.(string))
		if _, err := time.Parse("2006-01-02", s); err != nil {
			details = append(details, middleware.ValidationDetail{Field: "date_of_birth", Message: "must be YYYY-MM-DD"})
		} else {
			req.DateOfBirth = &s
		}
	}

	if v, ok := input["gender"]; ok {
		s := strings.TrimSpace(strings.ToLower(v.(string)))
		if s != "male" && s != "female" && s != "other" {
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

	if v, ok := input["current_city"]; ok {
		s := strings.TrimSpace(v.(string))
		if len(s) > 80 {
			details = append(details, middleware.ValidationDetail{Field: "current_city", Message: "must be at most 80 characters"})
		} else {
			req.CurrentCity = &s
		}
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

	if v, ok := input["line1"]; ok {
		s := strings.TrimSpace(v.(string))
		if s == "" || len(s) > 255 {
			details = append(details, middleware.ValidationDetail{Field: "line1", Message: "must be 1–255 characters"})
		} else {
			req.Line1 = &s
		}
	}
	if v, ok := input["city"]; ok {
		s := strings.TrimSpace(v.(string))
		if s == "" || len(s) > 100 {
			details = append(details, middleware.ValidationDetail{Field: "city", Message: "must be 1–100 characters"})
		} else {
			req.City = &s
		}
	}
	if v, ok := input["state"]; ok {
		s := strings.TrimSpace(v.(string))
		if s == "" || len(s) > 100 {
			details = append(details, middleware.ValidationDetail{Field: "state", Message: "must be 1–100 characters"})
		} else {
			req.State = &s
		}
	}
	if v, ok := input["pincode"]; ok {
		s := strings.TrimSpace(strings.ToUpper(v.(string)))
		if !rePincode.MatchString(s) {
			details = append(details, middleware.ValidationDetail{Field: "pincode", Message: "must be 3–10 alphanumeric characters"})
		} else {
			req.Pincode = &s
		}
	}

	if v, ok := input["line2"]; ok {
		s := strings.TrimSpace(v.(string))
		req.Line2 = &s
	}
	if v, ok := input["area"]; ok {
		s := strings.TrimSpace(v.(string))
		req.Area = &s
	}
	if v, ok := input["label"]; ok {
		s := strings.TrimSpace(v.(string))
		req.Label = &s
	}
	if v, ok := input["contact_name"]; ok {
		s := strings.TrimSpace(v.(string))
		req.ContactName = &s
	}
	if v, ok := input["contact_phone"]; ok {
		cp := strings.TrimSpace(v.(string))
		if cp != "" && !rePhone.MatchString(cp) {
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
