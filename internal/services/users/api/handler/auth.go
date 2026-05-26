package handler

import (
	"net/http"
	"strings"
	"time"

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
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error_code": "VALIDATION_ERROR", "message": "invalid request body", "details": []string{}})
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

	if out.Registered && out.AccountStatus == "suspended" {
		c.JSON(http.StatusOK, gin.H{
			"status":     "error",
			"error_code": "ACCOUNT_SUSPENDED",
			"data":       out,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": out})
}

func (h *AuthHandler) Register(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.RegisterRequest](c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error_code": "VALIDATION_ERROR", "message": "invalid request body", "details": []string{}})
		return
	}

	out, svcErr := h.svc.Register(c.Request.Context(), models.RegisterInput{
		Phone:        req.Phone,
		Name:         req.Name,
		Email:        req.Email,
		ReferralCode: req.ReferralCode,
		Role:         req.Role,
		DeviceID:     strings.TrimSpace(c.GetHeader("X-Device-ID")),
		IPAddress:    c.ClientIP(),
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": out})
}

func (h *AuthHandler) SendOTP(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.SendOTPRequest](c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error_code": "VALIDATION_ERROR", "message": "invalid request body", "details": []string{}})
		return
	}

	out, svcErr := h.svc.SendOTP(c.Request.Context(), models.SendOTPInput{
		Phone:     req.Phone,
		DeviceID:  strings.TrimSpace(c.GetHeader("X-Device-ID")),
		IPAddress: c.ClientIP(),
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": out})
}

func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.VerifyOTPRequest](c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error_code": "VALIDATION_ERROR", "message": "invalid request body", "details": []string{}})
		return
	}

	clientType := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Client-Type")))
	out, svcErr := h.svc.VerifyOTP(c.Request.Context(), models.VerifyOTPInput{
		Phone:      req.Phone,
		OTP:        req.OTP,
		DeviceID:   strings.TrimSpace(c.GetHeader("X-Device-ID")),
		IPAddress:  c.ClientIP(),
		Platform:   strings.ToLower(strings.TrimSpace(c.GetHeader("X-Platform"))),
		ClientType: clientType,
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}

	if clientType == "web" {
		c.SetCookie("refresh_token", out.RefreshToken, int((30 * time.Hour * 24).Seconds()), "/", "", true, true)
		out.RefreshToken = ""
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": out})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	sessionID, _ := c.Get(middleware.ContextSessionIDKey)
	userID, _ := c.Get(middleware.ContextUserIDKey)
	role, _ := c.Get(middleware.ContextRoleKey)

	svcErr := h.svc.Logout(c.Request.Context(), models.LogoutInput{
		SessionID: toString(sessionID),
		UserID:    toString(userID),
		Role:      toString(role),
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": gin.H{"message": "Logged out successfully"}})
}

func writeServiceError(c *gin.Context, err *models.ServiceError) {
	c.JSON(err.StatusCode, gin.H{
		"status":     "error",
		"error_code": err.Code,
		"message":    err.Message,
		"details":    err.Details,
	})
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}
