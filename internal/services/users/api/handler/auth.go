package handler

import (
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/users/business"
	"food-delivery-backend/internal/services/users/models"
	"food-delivery-backend/pkg/response"

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
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid request body", []string{})
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
		response.ErrorWithData(c, http.StatusOK, apperrors.CodeAccountSuspended, "account suspended", out, []string{})
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *AuthHandler) Register(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.RegisterRequest](c)
	if !ok {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid request body", []string{})
		return
	}

	sid, _ := c.Get(constants.GuestTokenSessionIDKey)
	out, svcErr := h.svc.Register(c.Request.Context(), models.RegisterInput{
		Phone:          req.Phone,
		Name:           req.Name,
		Email:          req.Email,
		ReferralCode:   req.ReferralCode,
		Role:           req.Role,
		DeviceID:       strings.TrimSpace(c.GetHeader(constants.HeaderDeviceID)),
		IPAddress:      c.ClientIP(),
		GuestSessionID: toString(sid),
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *AuthHandler) SendOTP(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.SendOTPRequest](c)
	if !ok {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid request body", []string{})
		return
	}

	out, svcErr := h.svc.SendOTP(c.Request.Context(), models.SendOTPInput{
		Phone:     req.Phone,
		Role:      req.Role,
		DeviceID:  strings.TrimSpace(c.GetHeader(constants.HeaderDeviceID)),
		IPAddress: c.ClientIP(),
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.VerifyOTPRequest](c)
	if !ok {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid request body", []string{})
		return
	}

	clientType := strings.ToLower(strings.TrimSpace(c.GetHeader(constants.HeaderClientType)))
	out, svcErr := h.svc.VerifyOTP(c.Request.Context(), models.VerifyOTPInput{
		Phone:      req.Phone,
		Role:       req.Role,
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

	response.Success(c, http.StatusOK, out)
}

func (h *AuthHandler) SendEmailOTP(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.SendEmailOTPRequest](c)
	if !ok {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid request body", []string{})
		return
	}

	sid, _ := c.Get(constants.GuestTokenSessionIDKey)
	out, svcErr := h.svc.SendEmailOTP(c.Request.Context(), models.SendEmailOTPInput{
		GuestSessionID: toString(sid),
		Email:          req.Email,
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.VerifyEmailRequest](c)
	if !ok {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid request body", []string{})
		return
	}

	sid, _ := c.Get(constants.GuestTokenSessionIDKey)
	out, svcErr := h.svc.VerifyEmail(c.Request.Context(), models.VerifyEmailInput{
		GuestSessionID: toString(sid),
		Email:          req.Email,
		OTP:            req.OTP,
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	response.Success(c, http.StatusOK, out)
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
	response.Success(c, http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func writeServiceError(c *gin.Context, err *models.ServiceError) {
	response.Error(c, err.StatusCode, err.Code, err.Message, err.Details)
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}
