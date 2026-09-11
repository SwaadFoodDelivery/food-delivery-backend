package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"food-delivery-backend/internal/constants"
	"food-delivery-backend/internal/services/operations/business"
	"food-delivery-backend/internal/services/operations/repository"
	"food-delivery-backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ svc business.Service }

func NewHandler(svc business.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Overview(c *gin.Context) {
	out, err := h.svc.GetOverview(c.Request.Context(), c.Query("status"))
	if err != nil {
		if strings.Contains(err.Error(), "unsupported order status") {
			response.Error(c, http.StatusBadRequest, "INVALID_ORDER_STATUS", err.Error(), []string{})
			return
		}
		response.Error(c, http.StatusInternalServerError, "OPERATIONS_LOOKUP_FAILED", "operations overview could not be loaded", []string{})
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *Handler) CancelOrder(c *gin.Context) {
	orderID, err := uuid.Parse(strings.TrimSpace(c.Param("orderId")))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ORDER_ID", "orderId must be a UUID", []string{})
		return
	}
	actorRaw, ok := c.Get(constants.AuthContextUserIDKey)
	actorID, parseErr := uuid.Parse(strings.TrimSpace(valueString(actorRaw)))
	if !ok || parseErr != nil {
		response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "invalid user identity", []string{})
		return
	}
	out, err := h.svc.CancelOrder(c.Request.Context(), actorID, orderID)
	if errors.Is(err, repository.ErrOrderNotFound) {
		response.Error(c, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found", []string{})
		return
	}
	if errors.Is(err, repository.ErrOrderNotCancelable) {
		response.Error(c, http.StatusConflict, "ORDER_NOT_CANCELABLE", "order cannot be cancelled in its current state", []string{})
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "ORDER_INTERVENTION_FAILED", "order intervention failed", []string{})
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *Handler) Audit(c *gin.Context) {
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_AUDIT_LIMIT", "limit must be a number between 1 and 100", []string{})
			return
		}
		limit = parsed
	}
	items, err := h.svc.ListAuditEvents(c.Request.Context(), c.Query("action"), c.Query("entity_type"), limit)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_AUDIT_LIMIT", err.Error(), []string{})
		return
	}
	response.Success(c, http.StatusOK, gin.H{"items": items})
}

func valueString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
