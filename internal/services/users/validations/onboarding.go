package validations

import (
	"strings"

	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/users/models"

	"github.com/gin-gonic/gin"
)

func ValidateInitOnboardingBody(input map[string]any, _ *gin.Context) (any, []middleware.ValidationDetail) {
	req := models.InitOnboardingRequest{}
	details := make([]middleware.ValidationDetail, 0)

	role := strings.TrimSpace(getString(input, "role"))
	if !validRole(role) {
		details = append(details, middleware.ValidationDetail{Field: "role", Message: "must be one of client, restaurant_owner, restaurant_manager, driver"})
	} else {
		req.Role = role
	}

	country := strings.ToUpper(strings.TrimSpace(getString(input, "country")))
	if country != "" && len(country) != 2 {
		details = append(details, middleware.ValidationDetail{Field: "country", Message: "must be 2-letter country code"})
	} else {
		req.Country = country
	}

	return req, details
}

func ValidateDocumentUploadedBody(input map[string]any, _ *gin.Context) (any, []middleware.ValidationDetail) {
	s3Key := strings.TrimSpace(getString(input, "s3_key"))
	details := make([]middleware.ValidationDetail, 0)
	if s3Key == "" {
		details = append(details, middleware.ValidationDetail{Field: "s3_key", Message: "is required"})
	}
	return map[string]string{"s3_key": s3Key}, details
}
