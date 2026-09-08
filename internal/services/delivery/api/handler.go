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
