package api

import (
	"context"
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	"food-delivery-backend/pkg/response"
	orderpb "github.com/SwaadFoodDelivery/proto/order"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrderReader interface {
	GetOrder(context.Context, *orderpb.GetOrderRequest) (*orderpb.OrderResponse, error)
}

// Delegate only middleware-authenticated identity, never HTTP requester fields.
func NewGRPCReadHandler(reader OrderReader) gin.HandlerFunc {
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
		id, err := uuid.Parse(strings.TrimSpace(c.Param("orderId")))
		if err != nil || id == uuid.Nil {
			response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "orderId must be a UUID", []string{})
			return
		}
		out, err := reader.GetOrder(c.Request.Context(), &orderpb.GetOrderRequest{OrderId: id.String(), RequesterUserId: uid.String(), RequesterRole: constants.RoleClient})
		if err != nil {
			httpStatus, code, message := http.StatusBadGateway, "ORDER_SERVICE_ERROR", "order service request failed"
			switch status.Code(err) {
			case codes.NotFound:
				httpStatus, code, message = http.StatusNotFound, "ORDER_NOT_FOUND", "order not found"
			case codes.InvalidArgument:
				httpStatus, code, message = http.StatusBadRequest, "VALIDATION_ERROR", "invalid order request"
			case codes.PermissionDenied:
				httpStatus, code, message = http.StatusForbidden, "FORBIDDEN", "order access denied"
			case codes.DeadlineExceeded:
				httpStatus, code, message = http.StatusGatewayTimeout, "ORDER_SERVICE_TIMEOUT", "order service timed out"
			case codes.Unavailable, codes.Canceled:
				httpStatus, code, message = http.StatusServiceUnavailable, "ORDER_SERVICE_UNAVAILABLE", "order service unavailable"
			}
			// Dependency auth errors are 502, not a request for customer re-login.
			response.Error(c, httpStatus, code, message, []string{})
			return
		}
		if out == nil {
			response.Error(c, http.StatusBadGateway, "ORDER_SERVICE_ERROR", "empty order service response", []string{})
			return
		}
		items := make([]gin.H, 0, len(out.Items))
		for _, item := range out.Items {
			if item == nil {
				continue
			}
			items = append(items, gin.H{"item_id": item.ItemId, "item_name_snapshot": item.ItemNameSnapshot, "quantity": item.Quantity, "item_price_snapshot_minor": item.ItemPriceSnapshotMinor, "line_total_minor": item.LineTotalMinor})
		}
		response.Success(c, http.StatusOK, gin.H{"order_id": out.OrderId, "status": strings.ToLower(out.Status.String()), "total_amount_minor": out.TotalAmountMinor, "currency": out.Currency, "created_at": out.CreatedAt, "restaurant_id": out.RestaurantId, "payment_status": out.PaymentStatus, "payment_method": out.PaymentMethod, "items": items})
	}
}
