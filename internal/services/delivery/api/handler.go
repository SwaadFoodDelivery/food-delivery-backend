package api

import (
	"errors"
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	"food-delivery-backend/internal/services/delivery/business"
	"food-delivery-backend/internal/services/delivery/repository"
	"food-delivery-backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ svc business.Service }

type driverStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func NewHandler(svc business.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Get(c *gin.Context) {
	orderID, err := uuid.Parse(strings.TrimSpace(c.Param("orderId")))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "order_id must be a UUID", []string{})
		return
	}
	userID, _ := c.Get(constants.AuthContextUserIDKey)
	uid, err := uuid.Parse(strings.TrimSpace(userID.(string)))
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "invalid user identity", []string{})
		return
	}
	out, err := h.svc.GetForUser(c.Request.Context(), uid, orderID)
	if errors.Is(err, repository.ErrDeliveryNotFound) {
		response.Error(c, http.StatusNotFound, "DELIVERY_NOT_FOUND", "delivery not found", []string{})
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "delivery lookup failed", []string{})
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *Handler) GetForDriver(c *gin.Context) {
	driverID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	out, err := h.svc.GetForDriver(c.Request.Context(), driverID)
	if errors.Is(err, repository.ErrDeliveryNotFound) {
		response.Error(c, http.StatusNotFound, "DELIVERY_NOT_FOUND", "delivery not found", []string{})
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "delivery lookup failed", []string{})
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *Handler) UpdateForDriver(c *gin.Context) {
	driverID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req driverStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_DELIVERY_STATUS", "status is required", []string{})
		return
	}
	out, err := h.svc.UpdateForDriver(c.Request.Context(), driverID, strings.TrimSpace(req.Status))
	if errors.Is(err, repository.ErrDeliveryNotFound) {
		response.Error(c, http.StatusNotFound, "DELIVERY_NOT_FOUND", "delivery not found", []string{})
		return
	}
	if errors.Is(err, repository.ErrInvalidTransition) {
		response.Error(c, http.StatusConflict, "INVALID_DELIVERY_TRANSITION", "delivery cannot move to that status", []string{})
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "delivery update failed", []string{})
		return
	}
	response.Success(c, http.StatusOK, out)
}

func authenticatedUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, ok := c.Get(constants.AuthContextUserIDKey)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "invalid user identity", []string{})
		return uuid.Nil, false
	}
	uid, err := uuid.Parse(strings.TrimSpace(valueString(userID)))
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "invalid user identity", []string{})
		return uuid.Nil, false
	}
	return uid, true
}

func valueString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
