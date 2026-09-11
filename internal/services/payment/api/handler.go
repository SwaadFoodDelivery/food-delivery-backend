package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/payment/business"
	"food-delivery-backend/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct{ svc business.Service }

func NewHandler(svc business.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Pay(c *gin.Context) {
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "Idempotency-Key header is required", []string{})
		return
	}
	var req payRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid JSON body", []string{})
		return
	}
	orderID := strings.TrimSpace(c.Param("orderId"))
	out, replay, err := h.svc.Pay(c.Request.Context(), business.Input{UserID: userID(c), OrderID: orderID, PaymentToken: req.PaymentToken, IdempotencyKey: key})
	if err != nil {
		writeError(c, err)
		return
	}
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	response.Success(c, status, mapPayment(out, replay))
}

func userID(c *gin.Context) string {
	value, _ := c.Get(constants.AuthContextUserIDKey)
	userID, _ := value.(string)
	return strings.TrimSpace(userID)
}

func writeError(c *gin.Context, err error) {
	var serviceErr *business.ServiceError
	if errors.As(err, &serviceErr) {
		response.Error(c, serviceErr.StatusCode, serviceErr.Code, serviceErr.Message, []string{})
		return
	}
	response.Error(c, http.StatusInternalServerError, apperrors.CodeInternal, "payment operation failed", []string{})
}
