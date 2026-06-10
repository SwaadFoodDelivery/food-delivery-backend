package validations

import (
	"strings"

	"food-delivery-backend/internal/constants"
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

func ValidateInitOnboardingInput(in models.InitOnboardingInput) (models.InitOnboardingInput, []string) {
	out := models.InitOnboardingInput{
		UserID:  strings.TrimSpace(in.UserID),
		Role:    strings.TrimSpace(in.Role),
		Country: strings.ToUpper(strings.TrimSpace(in.Country)),
	}
	details := make([]string, 0)
	if out.UserID == "" {
		details = append(details, "user_id is required")
	}
	if !validRole(out.Role) {
		details = append(details, "invalid role")
	}
	if out.Country == "" {
		out.Country = constants.DefaultCountryISO2
	}
	if len(out.Country) != 2 {
		details = append(details, "country must be ISO-2 code")
	}
	return out, details
}

func ValidateOnboardingIDs(userID, onboardingID string) (string, string, []string) {
	u := strings.TrimSpace(userID)
	o := strings.TrimSpace(onboardingID)
	details := make([]string, 0)
	if u == "" {
		details = append(details, "user_id is required")
	}
	if o == "" {
		details = append(details, "onboarding_id is required")
	}
	return u, o, details
}

func ValidateS3Key(s3Key string) (string, []string) {
	k := strings.TrimSpace(s3Key)
	if k == "" {
		return "", []string{"s3_key is required"}
	}
	return k, nil
}
