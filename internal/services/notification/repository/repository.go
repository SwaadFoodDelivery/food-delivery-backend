package repository

import (
	"context"
	"errors"

	"food-delivery-backend/internal/services/notification/models"
	"github.com/google/uuid"
)

var ErrNotificationNotFound = errors.New("notification not found")

type Repository interface {
	List(context.Context, uuid.UUID, bool, int) (models.ListResult, error)
	MarkRead(context.Context, uuid.UUID, uuid.UUID) (models.Notification, error)
	MarkAllRead(context.Context, uuid.UUID) (int, error)
}
