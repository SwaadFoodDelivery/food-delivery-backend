package business

import (
	"context"
	"testing"

	"food-delivery-backend/internal/services/restaurant/models"
	"food-delivery-backend/internal/services/restaurant/repository"
	"github.com/google/uuid"
)

type fakeRepository struct{}

func (fakeRepository) List(context.Context, repository.SearchFilter) (models.RestaurantList, error) {
	return models.RestaurantList{}, nil
}
func (fakeRepository) Get(context.Context, uuid.UUID, *float64, *float64) (models.Restaurant, error) {
	return models.Restaurant{}, nil
}
func (fakeRepository) Menu(context.Context, uuid.UUID) (models.Menu, error) {
	return models.Menu{}, nil
}

func TestListRejectsUnsafeBounds(t *testing.T) {
	svc := NewService(fakeRepository{})
	for name, filter := range map[string]repository.SearchFilter{
		"zero radius":  {RadiusKM: 0, Limit: 20},
		"large radius": {RadiusKM: 26, Limit: 20},
		"zero limit":   {RadiusKM: 5, Limit: 0},
		"large limit":  {RadiusKM: 5, Limit: 51},
		"bad sort":     {RadiusKM: 5, Limit: 20, SortBy: "created"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.List(context.Background(), filter); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
