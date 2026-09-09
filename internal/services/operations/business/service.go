package business

import (
	"context"
	"strings"

	"food-delivery-backend/internal/services/operations/models"
	"food-delivery-backend/internal/services/operations/repository"
	"github.com/google/uuid"
)

type Service interface {
	GetOverview(context.Context, string) (models.Overview, error)
	CancelOrder(context.Context, uuid.UUID, uuid.UUID) (models.Order, error)
}

type service struct{ repo repository.Repository }

func NewService(repo repository.Repository) Service { return &service{repo: repo} }

func (s *service) GetOverview(ctx context.Context, status string) (models.Overview, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if err := repositoryStatus(status); err != nil {
		return models.Overview{}, err
	}
	return s.repo.GetOverview(ctx, status)
}

func (s *service) CancelOrder(ctx context.Context, actorID, orderID uuid.UUID) (models.Order, error) {
	return s.repo.CancelOrder(ctx, actorID, orderID)
}

func repositoryStatus(status string) error {
	return repository.ValidateStatus(status)
}
