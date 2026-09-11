package repository

import (
	"context"
	"errors"

	"food-delivery-backend/internal/services/cart/models"
	"github.com/google/uuid"
)

var (
	ErrCartNotFound       = errors.New("cart not found")
	ErrRestaurantMismatch = errors.New("restaurant mismatch")
	ErrItemUnavailable    = errors.New("item unavailable")
	ErrItemNotFound       = errors.New("item not found")
	ErrCartItemNotFound   = errors.New("cart item not found")
)

type AddItemInput struct {
	UserID         uuid.UUID
	DeviceID       string
	CartID         *uuid.UUID
	RestaurantID   uuid.UUID
	ItemID         uuid.UUID
	Quantity       int
	Customisations []string
}

type AddItemOutput struct {
	CartID     uuid.UUID
	CartItemID uuid.UUID
}

type Repository interface {
	AddItem(context.Context, AddItemInput) (AddItemOutput, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (models.Cart, error)
	DeleteItem(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	Clear(context.Context, uuid.UUID, uuid.UUID) error
}
