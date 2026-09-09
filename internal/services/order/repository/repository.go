package repository

import (
	"context"
	"errors"

	cartmodels "food-delivery-backend/internal/services/cart/models"
	ordermodels "food-delivery-backend/internal/services/order/models"
	"github.com/google/uuid"
)

var (
	ErrCartEmpty          = errors.New("cart empty")
	ErrCartNotActive      = errors.New("cart not active")
	ErrItemPriceChanged   = errors.New("item price changed")
	ErrItemUnavailable    = errors.New("item unavailable")
	ErrAddressNotFound    = errors.New("address not found")
	ErrRestaurantNotFound = errors.New("restaurant not found")
	ErrNotServiceable     = errors.New("address not serviceable")
	ErrInvalidPayment     = errors.New("invalid payment method")
	ErrOrderNotFound      = errors.New("order not found")
	ErrOrderNotCancelable = errors.New("order cannot be cancelled")
)

type ServiceabilityRepository interface {
	CheckServiceability(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (ordermodels.Serviceability, error)
}

type Repository interface {
	Quote(context.Context, uuid.UUID, cartmodels.Cart, uuid.UUID) (ordermodels.Quote, error)
	FindByIdempotency(context.Context, uuid.UUID, string) (ordermodels.Order, bool, error)
	Place(context.Context, ordermodels.PlaceInput, cartmodels.Cart, ordermodels.Quote) (ordermodels.Order, bool, error)
	ServiceabilityRepository
}

type HistoryRepository interface {
	ListForUser(context.Context, uuid.UUID, int) ([]ordermodels.HistoryItem, error)
	GetHistory(context.Context, uuid.UUID, uuid.UUID) (ordermodels.History, error)
	CancelForUser(context.Context, uuid.UUID, uuid.UUID) (ordermodels.HistoryItem, error)
}
