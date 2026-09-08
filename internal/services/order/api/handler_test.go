package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"food-delivery-backend/internal/services/order/business"
	ordermodels "food-delivery-backend/internal/services/order/models"
	"github.com/gin-gonic/gin"
)

type fakeService struct{}

func (fakeService) Quote(context.Context, business.QuoteInput) (ordermodels.Quote, error) {
	return ordermodels.Quote{}, nil
}
func (fakeService) Place(context.Context, business.PlaceInput) (ordermodels.Order, bool, error) {
	return ordermodels.Order{}, false, nil
}

func TestPlaceRequiresIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/orders", NewHandler(fakeService{}).Place)
	req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{}`))
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", resp.Code)
	}
}
