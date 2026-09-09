package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/order/business"
	ordermodels "food-delivery-backend/internal/services/order/models"
	"food-delivery-backend/internal/services/order/repository"
	"food-delivery-backend/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type checkoutService interface {
	Quote(context.Context, business.QuoteInput) (ordermodels.Quote, error)
	Place(context.Context, business.PlaceInput) (ordermodels.Order, bool, error)
}

type historyService interface {
	List(context.Context, uuid.UUID, int) ([]ordermodels.HistoryItem, error)
	History(context.Context, uuid.UUID, uuid.UUID) (ordermodels.History, error)
}

type Handler struct{ svc checkoutService }

func NewHandler(svc checkoutService) *Handler { return &Handler{svc: svc} }

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

func (h *Handler) List(c *gin.Context) {
	limit := 20
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "limit must be a number between 1 and 50", []string{})
			return
		}
		limit = parsed
	}
	uid, err := uuid.Parse(userID(c))
	if err != nil {
		response.Error(c, http.StatusUnauthorized, apperrors.CodeInvalidToken, "invalid user identity", []string{})
		return
	}
	history, ok := h.svc.(historyService)
	if !ok {
		response.Error(c, http.StatusInternalServerError, "ORDER_HISTORY_UNAVAILABLE", "order history is unavailable", []string{})
		return
	}
	out, err := history.List(c.Request.Context(), uid, limit)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"orders": out})
}

func (h *Handler) History(c *gin.Context) {
	uid, err := uuid.Parse(userID(c))
	if err != nil {
		response.Error(c, http.StatusUnauthorized, apperrors.CodeInvalidToken, "invalid user identity", []string{})
		return
	}
	orderID, err := uuid.Parse(strings.TrimSpace(c.Param("orderId")))
	if err != nil {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "orderId must be a UUID", []string{})
		return
	}
	history, ok := h.svc.(historyService)
	if !ok {
		response.Error(c, http.StatusInternalServerError, "ORDER_HISTORY_UNAVAILABLE", "order history is unavailable", []string{})
		return
	}
	out, err := history.History(c.Request.Context(), uid, orderID)
	if errors.Is(err, repository.ErrOrderNotFound) {
		response.Error(c, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found", []string{})
		return
	}
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, out)
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
