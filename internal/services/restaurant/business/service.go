package business

import (
	"context"
	"fmt"
	"strings"

	"food-delivery-backend/internal/services/restaurant/models"
	"food-delivery-backend/internal/services/restaurant/repository"
	"github.com/google/uuid"
)

type Service interface {
	List(context.Context, repository.SearchFilter) (models.RestaurantList, error)
	Get(context.Context, uuid.UUID, *float64, *float64) (models.Restaurant, error)
	Menu(context.Context, uuid.UUID) (models.Menu, error)
}

type service struct{ repo repository.Repository }

func NewService(repo repository.Repository) Service { return &service{repo: repo} }

func (s *service) List(ctx context.Context, filter repository.SearchFilter) (models.RestaurantList, error) {
	if filter.RadiusKM <= 0 || filter.RadiusKM > 25 {
		return models.RestaurantList{}, fmt.Errorf("radius_km must be between 0 and 25")
	}
	if filter.Limit <= 0 || filter.Limit > 50 {
		return models.RestaurantList{}, fmt.Errorf("limit must be between 1 and 50")
	}
	if filter.Offset < 0 {
		return models.RestaurantList{}, fmt.Errorf("offset must be non-negative")
	}
	filter.Cuisine = strings.TrimSpace(filter.Cuisine)
	if filter.SortBy != "" && filter.SortBy != "rating" && filter.SortBy != "distance" {
		return models.RestaurantList{}, fmt.Errorf("sort_by must be rating or distance")
	}
	return s.repo.List(ctx, filter)
}

func (s *service) Get(ctx context.Context, restaurantID uuid.UUID, latitude, longitude *float64) (models.Restaurant, error) {
	return s.repo.Get(ctx, restaurantID, latitude, longitude)
}

func (s *service) Menu(ctx context.Context, restaurantID uuid.UUID) (models.Menu, error) {
	return s.repo.Menu(ctx, restaurantID)
}
