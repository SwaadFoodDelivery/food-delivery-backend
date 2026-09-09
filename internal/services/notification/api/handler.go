package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"food-delivery-backend/internal/constants"
	"food-delivery-backend/internal/services/notification/business"
	"food-delivery-backend/internal/services/notification/repository"
	"food-delivery-backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ svc business.Service }

func NewHandler(svc business.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) List(c *gin.Context) {
	recipientID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	unreadOnly := strings.EqualFold(strings.TrimSpace(c.Query("unread")), "true")
	limit := 20
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_LIMIT", "limit must be a number between 1 and 50", []string{})
			return
		}
		limit = parsed
	}
	out, err := h.svc.List(c.Request.Context(), recipientID, unreadOnly, limit)
	if err != nil {
		if strings.Contains(err.Error(), "notification limit") {
			response.Error(c, http.StatusBadRequest, "INVALID_LIMIT", err.Error(), []string{})
			return
		}
		response.Error(c, http.StatusInternalServerError, "NOTIFICATIONS_LOOKUP_FAILED", "notifications could not be loaded", []string{})
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *Handler) MarkRead(c *gin.Context) {
	recipientID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	notificationID, err := uuid.Parse(strings.TrimSpace(c.Param("notificationId")))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_NOTIFICATION_ID", "notificationId must be a UUID", []string{})
		return
	}
	out, err := h.svc.MarkRead(c.Request.Context(), recipientID, notificationID)
	if errors.Is(err, repository.ErrNotificationNotFound) {
		response.Error(c, http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "notification not found", []string{})
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "NOTIFICATION_UPDATE_FAILED", "notification could not be updated", []string{})
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *Handler) MarkAllRead(c *gin.Context) {
	recipientID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	count, err := h.svc.MarkAllRead(c.Request.Context(), recipientID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "NOTIFICATION_UPDATE_FAILED", "notifications could not be updated", []string{})
		return
	}
	response.Success(c, http.StatusOK, gin.H{"marked_read": count})
}

func authenticatedUserID(c *gin.Context) (uuid.UUID, bool) {
	raw, ok := c.Get(constants.AuthContextUserIDKey)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "invalid user identity", []string{})
		return uuid.Nil, false
	}
	value, valid := raw.(string)
	if !valid {
		response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "invalid user identity", []string{})
		return uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "invalid user identity", []string{})
		return uuid.Nil, false
	}
	return id, true
}
