package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"food-delivery-backend/internal/constants"
	"food-delivery-backend/pkg/response"
	orderpb "github.com/SwaadFoodDelivery/proto/order"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OrderLister interface {
	GetUserOrders(context.Context, *orderpb.GetUserOrdersRequest) (*orderpb.GetUserOrdersResponse, error)
}

// NewGRPCListHandler preserves the existing summary envelope, with an additive
// next_cursor. Identity always comes from authentication middleware, not query fields.
func NewGRPCListHandler(reader OrderLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, err := uuid.Parse(userID(c))
		if err != nil || uid == uuid.Nil {
			response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "invalid user identity", []string{})
			return
		}
		if c.GetString(constants.AuthContextRoleKey) != constants.RoleClient {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "client access required", []string{})
			return
		}
		limit := 20
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			limit, err = strconv.Atoi(raw)
		}
		cursor := c.Query("cursor")
		if err != nil || limit < 1 || limit > 50 || len(cursor) > 1024 {
			response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "limit must be 1 to 50 and cursor at most 1024 bytes", []string{})
			return
		}
		out, err := reader.GetUserOrders(c.Request.Context(), &orderpb.GetUserOrdersRequest{UserId: uid.String(), RequesterRole: constants.RoleClient, Limit: int32(limit), Cursor: cursor})
		if err != nil {
			writeOrderRPCError(c, err)
			return
		}
		if out == nil || len(out.Orders) > limit {
			response.Error(c, http.StatusBadGateway, "ORDER_SERVICE_ERROR", "invalid order service response", []string{})
			return
		}
		orders := make([]gin.H, 0, len(out.Orders))
		for _, order := range out.Orders {
			if order == nil {
				response.Error(c, http.StatusBadGateway, "ORDER_SERVICE_ERROR", "invalid order service response", []string{})
				return
			}
			summary := gin.H{"order_id": order.OrderId, "status": strings.ToLower(order.Status.String()), "restaurant_name": order.RestaurantName, "total_amount_minor": order.TotalAmountMinor, "currency": order.Currency, "payment_method": order.PaymentMethod, "created_at": order.CreatedAt}
			if order.DeliveryStatus != "" {
				summary["delivery_status"] = order.DeliveryStatus
			}
			orders = append(orders, summary)
		}
		response.Success(c, http.StatusOK, gin.H{"orders": orders, "next_cursor": out.NextCursor})
	}
}
