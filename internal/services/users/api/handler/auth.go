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

type AuthHandler struct {
	svc business.AuthService
}

func NewAuthHandler(svc business.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func (h *AuthHandler) CheckPhone(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.CheckPhoneRequest](c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"status": constants.ResponseStatusError, "error_code": apperrors.CodeValidation, "message": "invalid request body", "details": []string{}})
		return
	}

	out, svcErr := h.svc.CheckPhone(c.Request.Context(), models.CheckPhoneInput{
		Phone: req.Phone,
		Role:  req.Role,
		IP:    c.ClientIP(),
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}

	if out.Registered && out.AccountStatus == constants.AccountStatusSuspended {
		c.JSON(http.StatusOK, gin.H{
			"status":     constants.ResponseStatusError,
			"error_code": apperrors.CodeAccountSuspended,
			"data":       out,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": constants.ResponseStatusSuccess, "data": out})
}

func (h *AuthHandler) Register(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.RegisterRequest](c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"status": constants.ResponseStatusError, "error_code": apperrors.CodeValidation, "message": "invalid request body", "details": []string{}})
		return
	}

	out, svcErr := h.svc.Register(c.Request.Context(), models.RegisterInput{
		Phone:        req.Phone,
		Name:         req.Name,
		Email:        req.Email,
		ReferralCode: req.ReferralCode,
		Role:         req.Role,
		DeviceID:     strings.TrimSpace(c.GetHeader(constants.HeaderDeviceID)),
		IPAddress:    c.ClientIP(),
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": constants.ResponseStatusSuccess, "data": out})
}

func (h *AuthHandler) SendOTP(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.SendOTPRequest](c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"status": constants.ResponseStatusError, "error_code": apperrors.CodeValidation, "message": "invalid request body", "details": []string{}})
		return
	}

	out, svcErr := h.svc.SendOTP(c.Request.Context(), models.SendOTPInput{
		Phone:     req.Phone,
		DeviceID:  strings.TrimSpace(c.GetHeader(constants.HeaderDeviceID)),
		IPAddress: c.ClientIP(),
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": constants.ResponseStatusSuccess, "data": out})
}

func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.VerifyOTPRequest](c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"status": constants.ResponseStatusError, "error_code": apperrors.CodeValidation, "message": "invalid request body", "details": []string{}})
		return
	}

	clientType := strings.ToLower(strings.TrimSpace(c.GetHeader(constants.HeaderClientType)))
	out, svcErr := h.svc.VerifyOTP(c.Request.Context(), models.VerifyOTPInput{
		Phone:      req.Phone,
		OTP:        req.OTP,
		DeviceID:   strings.TrimSpace(c.GetHeader(constants.HeaderDeviceID)),
		IPAddress:  c.ClientIP(),
		Platform:   strings.ToLower(strings.TrimSpace(c.GetHeader(constants.HeaderPlatform))),
		ClientType: clientType,
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}

	if clientType == constants.PlatformWeb {
		c.SetCookie("refresh_token", out.RefreshToken, int(constants.AuthRefreshTokenTTL.Seconds()), "/", "", true, true)
		out.RefreshToken = ""
	}

	c.JSON(http.StatusOK, gin.H{"status": constants.ResponseStatusSuccess, "data": out})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	sessionID, _ := c.Get(constants.AuthContextSessionIDKey)
	userID, _ := c.Get(constants.AuthContextUserIDKey)
	role, _ := c.Get(constants.AuthContextRoleKey)

	svcErr := h.svc.Logout(c.Request.Context(), models.LogoutInput{
		SessionID: toString(sessionID),
		UserID:    toString(userID),
		Role:      toString(role),
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": constants.ResponseStatusSuccess, "data": gin.H{"message": "Logged out successfully"}})
}

func writeServiceError(c *gin.Context, err *models.ServiceError) {
	c.JSON(err.StatusCode, gin.H{
		"status":     constants.ResponseStatusError,
		"error_code": err.Code,
		"message":    err.Message,
		"details":    err.Details,
	})
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}
