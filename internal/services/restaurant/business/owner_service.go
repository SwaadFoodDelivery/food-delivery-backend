package business

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"food-delivery-backend/internal/services/restaurant/models"
	"food-delivery-backend/internal/services/restaurant/repository"
	"github.com/google/uuid"
)

type OwnerService interface {
	CreateItem(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, models.Item) (models.Item, error)
	UpdateItem(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, models.Item) (models.Item, error)
	DeleteItem(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
}

type ownerService struct{ repo repository.OwnerRepository }

func NewOwnerService(repo repository.OwnerRepository) OwnerService { return &ownerService{repo: repo} }

func (s *ownerService) CreateItem(ctx context.Context, actorID, restaurantID, categoryID uuid.UUID, item models.Item) (models.Item, error) {
	if err := validateItem(item); err != nil {
		return models.Item{}, err
	}
	return s.repo.CreateItem(ctx, actorID, restaurantID, categoryID, item)
}

func (s *ownerService) UpdateItem(ctx context.Context, actorID, restaurantID, itemID uuid.UUID, item models.Item) (models.Item, error) {
	if err := validateItem(item); err != nil {
		return models.Item{}, err
	}
	return s.repo.UpdateItem(ctx, actorID, restaurantID, itemID, item)
}

func (s *ownerService) DeleteItem(ctx context.Context, actorID, restaurantID, itemID uuid.UUID) error {
	return s.repo.DeleteItem(ctx, actorID, restaurantID, itemID)
}

func validateItem(item models.Item) error {
	if strings.TrimSpace(item.Name) == "" || len(item.Name) > 200 {
		return fmt.Errorf("name is required and must be at most 200 characters")
	}
	if item.Price <= 0 {
		return fmt.Errorf("price_minor must be positive")
	}
	if item.PreparationTimeMin < 0 || item.PreparationTimeMin > 1440 {
		return fmt.Errorf("preparation_time_min must be between 0 and 1440")
	}
	if item.ImageURL != "" {
		u, err := url.Parse(item.ImageURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "s3") {
			return fmt.Errorf("image_url must use https or s3")
		}
	}
	return nil
}
