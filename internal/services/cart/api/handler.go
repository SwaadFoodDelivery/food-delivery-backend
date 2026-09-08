package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/cart/business"
	"food-delivery-backend/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ svc business.Service }

func NewHandler(svc business.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) AddItem(c *gin.Context) {
	var req addItemRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid JSON body", []string{})
		return
	}
	restaurantID, itemID, err := req.IDs()
	if err != nil || req.Quantity < 1 {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "restaurant_id and item_id must be UUIDs and quantity must be positive", []string{})
		return
	}
	userID := contextString(c, constants.AuthContextUserIDKey)
	deviceID := strings.TrimSpace(c.GetHeader(constants.HeaderDeviceID))
	out, err := h.svc.AddItem(c.Request.Context(), business.AddItemInput{UserID: userID, DeviceID: deviceID, CartToken: strings.TrimSpace(req.CartToken), RestaurantID: restaurantID, ItemID: itemID, Quantity: req.Quantity, Customisations: req.Customisations})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, addItemResponse{CartToken: out.CartToken, CartItemID: out.CartItemID.String(), Message: "Item added to cart"})
}

func (h *Handler) Get(c *gin.Context) {
	out, err := h.svc.Get(c.Request.Context(), business.GetInput{UserID: contextString(c, constants.AuthContextUserIDKey), CartToken: c.Param("cartToken")})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *Handler) DeleteItem(c *gin.Context) {
	itemID, err := uuid.Parse(c.Param("cartItemId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_CART_ITEM_ID", "cartItemId must be a UUID", []string{})
		return
	}
	if err := h.svc.DeleteItem(c.Request.Context(), business.DeleteItemInput{UserID: contextString(c, constants.AuthContextUserIDKey), CartToken: c.Param("cartToken"), CartItemID: itemID}); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"cart_item_id": itemID.String(), "message": "Cart item cleared successfully"})
}

func (h *Handler) Clear(c *gin.Context) {
	if err := h.svc.Clear(c.Request.Context(), business.ClearInput{UserID: contextString(c, constants.AuthContextUserIDKey), CartToken: c.Param("cartToken")}); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"cart_token": c.Param("cartToken"), "message": "All items deleted from cart successfully"})
}

func contextString(c *gin.Context, key string) string {
	value, _ := c.Get(key)
	return strings.TrimSpace(valueString(value))
}

func valueString(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func writeError(c *gin.Context, err error) {
	var serviceErr *business.ServiceError
	if errors.As(err, &serviceErr) {
		response.Error(c, serviceErr.StatusCode, serviceErr.Code, serviceErr.Message, []string{})
		return
	}
	response.Error(c, http.StatusInternalServerError, apperrors.CodeInternal, "cart operation failed", []string{})
}
