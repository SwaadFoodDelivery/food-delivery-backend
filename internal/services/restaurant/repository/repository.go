package repository

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"

	"food-delivery-backend/internal/services/restaurant/models"
	"github.com/google/uuid"
)

type SearchFilter struct {
	Latitude  float64
	Longitude float64
	RadiusKM  float64
	Cuisine   string
	SortBy    string
	Offset    int
	Limit     int
}

type Repository interface {
	List(context.Context, SearchFilter) (models.RestaurantList, error)
	Get(context.Context, uuid.UUID, *float64, *float64) (models.Restaurant, error)
	Menu(context.Context, uuid.UUID) (models.Menu, error)
}

type OwnerRepository interface {
	CreateItem(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, models.Item) (models.Item, error)
	UpdateItem(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, models.Item) (models.Item, error)
	DeleteItem(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	ListOrders(context.Context, uuid.UUID, uuid.UUID) ([]models.RestaurantOrder, error)
	UpdateOrderStatus(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (models.RestaurantOrder, error)
}

func DecodeCursor(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, err
	}
	offset, err := strconv.Atoi(string(b))
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("invalid cursor")
	}
	return offset, nil
}

func EncodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}
