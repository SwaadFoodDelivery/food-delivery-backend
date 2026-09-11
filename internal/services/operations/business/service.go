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
	ListAuditEvents(context.Context, string, string, int) ([]models.AuditEvent, error)
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

func (s *service) ListAuditEvents(ctx context.Context, action, entityType string, limit int) ([]models.AuditEvent, error) {
	if err := repository.ValidateAuditLimit(limit); err != nil {
		return nil, err
	}
	return s.repo.ListAuditEvents(ctx, strings.TrimSpace(action), strings.TrimSpace(entityType), limit)
}

func repositoryStatus(status string) error {
	return repository.ValidateStatus(status)
}
