package business

import (
	"context"
	"time"

	"food-delivery-backend/internal/services/delivery/models"
	"food-delivery-backend/internal/services/delivery/repository"
	"github.com/google/uuid"
)

type Service interface {
	EnsureForOrder(context.Context, uuid.UUID, time.Time) error
	GetForUser(context.Context, uuid.UUID, uuid.UUID) (models.Delivery, error)
	GetForDriver(context.Context, uuid.UUID) (models.Delivery, error)
	UpdateForDriver(context.Context, uuid.UUID, string) (models.Delivery, error)
	Run(context.Context)
}

type MockService struct {
	repo     repository.Repository
	duration time.Duration
}

func NewMockService(repo repository.Repository, duration time.Duration) *MockService {
	if duration <= 0 {
		duration = 10 * time.Minute
	}
	return &MockService{repo: repo, duration: duration}
}

func (s *MockService) EnsureForOrder(ctx context.Context, orderID uuid.UUID, createdAt time.Time) error {
	_, err := s.repo.EnsureMockDelivery(ctx, orderID, createdAt, s.duration)
	return err
}

func (s *MockService) GetForUser(ctx context.Context, userID, orderID uuid.UUID) (models.Delivery, error) {
	return s.repo.GetForUser(ctx, userID, orderID)
}

func (s *MockService) GetForDriver(ctx context.Context, driverID uuid.UUID) (models.Delivery, error) {
	return s.repo.GetForDriver(ctx, driverID)
}

func (s *MockService) UpdateForDriver(ctx context.Context, driverID uuid.UUID, next string) (models.Delivery, error) {
	return s.repo.UpdateForDriver(ctx, driverID, next, s.duration)
}

func (s *MockService) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			_ = s.repo.AdvanceMockDeliveries(ctx, now.UTC(), s.duration)
		}
	}
}
