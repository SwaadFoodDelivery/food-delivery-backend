package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/order/business"
	"food-delivery-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc business.Service }

func NewHandler(svc business.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Quote(c *gin.Context) {
	var req quoteRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid JSON body", []string{})
		return
	}
	out, err := h.svc.Quote(c.Request.Context(), business.QuoteInput{UserID: userID(c), CartToken: req.CartToken, AddressID: req.AddressID})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *Handler) Place(c *gin.Context) {
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "Idempotency-Key header is required", []string{})
		return
	}
	var req placeRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid JSON body", []string{})
		return
	}
	out, replay, err := h.svc.Place(c.Request.Context(), business.PlaceInput{UserID: userID(c), CartToken: req.CartToken, AddressID: req.AddressID, PaymentMethod: req.PaymentMethod, Instructions: req.Instructions, IdempotencyKey: key})
	if err != nil {
		writeError(c, err)
		return
	}
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	response.Success(c, status, placeResponse{OrderID: out.OrderID.String(), Status: out.Status, TotalAmount: out.TotalAmount, Currency: out.Currency, EstimatedDelivery: out.EstimatedDelivery.Format("2006-01-02T15:04:05Z07:00"), Replay: replay})
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
	response.Error(c, http.StatusInternalServerError, apperrors.CodeInternal, "order operation failed", []string{})
}
