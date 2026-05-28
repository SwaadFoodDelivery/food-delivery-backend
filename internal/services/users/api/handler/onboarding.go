package handler

import (
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/users/business"
	"food-delivery-backend/internal/services/users/models"

	"github.com/gin-gonic/gin"
)

type OnboardingHandler struct {
	svc business.OnboardingService
}

func NewOnboardingHandler(svc business.OnboardingService) *OnboardingHandler {
	return &OnboardingHandler{svc: svc}
}

func (h *OnboardingHandler) Init(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.InitOnboardingRequest](c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"status": constants.ResponseStatusError, "error_code": apperrors.CodeValidation, "message": "invalid request body", "details": []string{}})
		return
	}

	userID, _ := c.Get(constants.AuthContextUserIDKey)
	role, _ := c.Get(constants.AuthContextRoleKey)
	if strings.TrimSpace(req.Role) != "" && strings.TrimSpace(req.Role) != strings.TrimSpace(toString(role)) {
		c.JSON(http.StatusForbidden, gin.H{"status": constants.ResponseStatusError, "error_code": apperrors.CodeRoleMismatch, "message": "requested role does not match authenticated role", "details": []string{}})
		return
	}

	out, svcErr := h.svc.InitOnboarding(c.Request.Context(), models.InitOnboardingInput{
		UserID:  strings.TrimSpace(toString(userID)),
		Role:    strings.TrimSpace(req.Role),
		Country: strings.TrimSpace(req.Country),
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": constants.ResponseStatusSuccess, "data": out})
}

func (h *OnboardingHandler) Submit(c *gin.Context) {
	userID, _ := c.Get(constants.AuthContextUserIDKey)
	onboardingID := strings.TrimSpace(c.Param("id"))

	out, svcErr := h.svc.SubmitOnboarding(c.Request.Context(), models.SubmitOnboardingInput{
		UserID:       strings.TrimSpace(toString(userID)),
		OnboardingID: onboardingID,
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": constants.ResponseStatusSuccess, "data": out})
}

func (h *OnboardingHandler) Resubmit(c *gin.Context) {
	userID, _ := c.Get(constants.AuthContextUserIDKey)
	onboardingID := strings.TrimSpace(c.Param("id"))

	out, svcErr := h.svc.ResubmitOnboarding(c.Request.Context(), models.ResubmitOnboardingInput{
		UserID:       strings.TrimSpace(toString(userID)),
		OnboardingID: onboardingID,
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": constants.ResponseStatusSuccess, "data": out})
}

func (h *OnboardingHandler) MarkUploaded(c *gin.Context) {
	body, ok := middleware.GetValidatedBody[map[string]string](c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"status": constants.ResponseStatusError, "error_code": apperrors.CodeValidation, "message": "invalid request body", "details": []string{}})
		return
	}

	svcErr := h.svc.MarkDocumentUploaded(c.Request.Context(), models.MarkDocumentUploadedInput{S3Key: strings.TrimSpace(body["s3_key"])})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": constants.ResponseStatusSuccess, "data": gin.H{"updated": true}})
}
