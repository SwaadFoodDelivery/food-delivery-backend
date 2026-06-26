package handler

import (
	"net/http"
	"strings"
	"time"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/pkg/config"
	"food-delivery-backend/pkg/response"
	apputils "food-delivery-backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SessionHandler struct {
	cfg *config.Config
}

func NewSessionHandler(cfg *config.Config) *SessionHandler {
	return &SessionHandler{cfg: cfg}
}

// InitSession issues a short-lived guest token bound to the caller's device ID.
// All public auth endpoints require this token via X-Guest-Token header.
func (h *SessionHandler) InitSession(c *gin.Context) {
	if h.cfg.JWT.GuestTokenSecret == "" {
		response.AbortError(c, http.StatusServiceUnavailable, apperrors.CodeGuestTokenInvalid, "guest token not configured", []string{})
		return
	}

	deviceID := strings.TrimSpace(c.GetHeader(constants.HeaderDeviceID))
	if deviceID == "" || len(deviceID) > 255 {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "X-Device-ID header is required and max length is 255", []string{})
		return
	}

	sessionID := uuid.NewString()
	ttl := time.Duration(h.cfg.JWT.GuestTokenTTLMin) * time.Minute

	token, err := apputils.CreateGuestToken(h.cfg.JWT.GuestTokenSecret, sessionID, deviceID, ttl)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, apperrors.CodeGuestTokenInvalid, "failed to issue guest token", []string{})
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"guest_token": token,
		"expires_in":  int(ttl.Seconds()),
		"session_id":  sessionID,
	})
}
