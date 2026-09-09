package business

import (
	"context"
	"fmt"

	"food-delivery-backend/internal/services/notification/models"
	"food-delivery-backend/internal/services/notification/repository"
	"github.com/google/uuid"
)

type Service interface {
	List(context.Context, uuid.UUID, bool, int) (models.ListResult, error)
	MarkRead(context.Context, uuid.UUID, uuid.UUID) (models.Notification, error)
	MarkAllRead(context.Context, uuid.UUID) (int, error)
}

type service struct{ repo repository.Repository }

func NewService(repo repository.Repository) Service { return &service{repo: repo} }

func (s *service) List(ctx context.Context, recipientID uuid.UUID, unreadOnly bool, limit int) (models.ListResult, error) {
	if limit < 1 || limit > 50 {
		return models.ListResult{}, fmt.Errorf("notification limit must be between 1 and 50")
	}
	return s.repo.List(ctx, recipientID, unreadOnly, limit)
}

func (s *service) MarkRead(ctx context.Context, recipientID, notificationID uuid.UUID) (models.Notification, error) {
	return s.repo.MarkRead(ctx, recipientID, notificationID)
}

func (s *service) MarkAllRead(ctx context.Context, recipientID uuid.UUID) (int, error) {
	return s.repo.MarkAllRead(ctx, recipientID)
}
