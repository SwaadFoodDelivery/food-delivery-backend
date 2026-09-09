package repository

import (
	"context"
	"errors"

	"food-delivery-backend/internal/services/operations/models"
	"github.com/google/uuid"
)

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrOrderNotCancelable = errors.New("order cannot be cancelled")
)

type Repository interface {
	GetOverview(context.Context, string) (models.Overview, error)
	CancelOrder(context.Context, uuid.UUID, uuid.UUID) (models.Order, error)
	ListAuditEvents(context.Context, string, string, int) ([]models.AuditEvent, error)
}
