package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"food-delivery-backend/internal/services/restaurant/models"
	"food-delivery-backend/internal/services/restaurant/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeService struct{}

func (fakeService) List(context.Context, repository.SearchFilter) (models.RestaurantList, error) {
	return models.RestaurantList{}, nil
}
func (fakeService) Get(context.Context, uuid.UUID, *float64, *float64) (models.Restaurant, error) {
	return models.Restaurant{}, nil
}
func (fakeService) Menu(context.Context, uuid.UUID) (models.Menu, error) { return models.Menu{}, nil }

func TestListRequiresCoordinates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/restaurants", NewHandler(fakeService{}).List)
	req := httptest.NewRequest(http.MethodGet, "/restaurants", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", resp.Code)
	}
}

func TestGetRejectsMalformedRestaurantID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/restaurants/:restaurantId", NewHandler(fakeService{}).Get)
	req := httptest.NewRequest(http.MethodGet, "/restaurants/not-a-uuid", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", resp.Code)
	}
}
