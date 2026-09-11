package repository

import (
	"context"
	"errors"

	"food-delivery-backend/internal/services/payment/models"
	"github.com/google/uuid"
)

var (
	ErrOrderNotFound       = errors.New("order not found")
	ErrOrderNotPayable     = errors.New("order is not payable")
	ErrPaymentNotFound     = errors.New("payment not found")
	ErrPaymentInProgress   = errors.New("payment already in progress")
	ErrIdempotencyConflict = errors.New("payment idempotency key belongs to another order")
)

type Repository interface {
	FindOrderForPayment(context.Context, uuid.UUID, uuid.UUID) (models.Order, error)
	FindByIdempotency(context.Context, string) (models.Payment, bool, error)
	CreatePending(context.Context, models.Payment, string) (models.Payment, bool, error)
	Complete(context.Context, uuid.UUID, string, string, string, string) (models.Payment, error)
}
