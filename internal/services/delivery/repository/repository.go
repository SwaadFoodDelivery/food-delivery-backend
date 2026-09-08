package repository

import (
	"context"
	"errors"
	"time"

	"food-delivery-backend/internal/services/delivery/models"
	"github.com/google/uuid"
)

var (
	ErrNoDemoPartner     = errors.New("no available demo delivery partner")
	ErrDeliveryNotFound  = errors.New("delivery not found")
	ErrInvalidTransition = errors.New("invalid driver delivery transition")
)

type Repository interface {
	EnsureMockDelivery(context.Context, uuid.UUID, time.Time, time.Duration) (models.Delivery, error)
	AdvanceMockDeliveries(context.Context, time.Time, time.Duration) error
	GetForUser(context.Context, uuid.UUID, uuid.UUID) (models.Delivery, error)
	GetForDriver(context.Context, uuid.UUID) (models.Delivery, error)
	UpdateForDriver(context.Context, uuid.UUID, string, time.Duration) (models.Delivery, error)
}
