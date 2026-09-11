package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"food-delivery-backend/internal/constants"
	"food-delivery-backend/internal/services/cart/business"
	"food-delivery-backend/internal/services/cart/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeService struct{ add business.AddItemOutput }

func (f fakeService) AddItem(context.Context, business.AddItemInput) (business.AddItemOutput, error) {
	return f.add, nil
}
func (fakeService) Get(context.Context, business.GetInput) (models.Cart, error) {
	return models.Cart{Items: []models.Item{}, Currency: "INR"}, nil
}
func (fakeService) DeleteItem(context.Context, business.DeleteItemInput) error { return nil }
func (fakeService) Clear(context.Context, business.ClearInput) error           { return nil }

func TestAddItemRejectsMalformedJSONAndIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/cart", func(c *gin.Context) {
		c.Set(constants.AuthContextUserIDKey, uuid.New().String())
		NewHandler(fakeService{}).AddItem(c)
	})
	for _, body := range []string{"{", `{"restaurant_id":"bad","item_id":"bad","quantity":1}`} {
		req := httptest.NewRequest(http.MethodPost, "/cart", strings.NewReader(body))
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("body %q: got %d, want 400", body, resp.Code)
		}
	}
}

func TestAddItemReturnsCreatedCartToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	itemID := uuid.New()
	r := gin.New()
	r.POST("/cart", func(c *gin.Context) {
		c.Set(constants.AuthContextUserIDKey, uuid.New().String())
		NewHandler(fakeService{add: business.AddItemOutput{CartToken: "cart.token.sig", CartItemID: itemID}}).AddItem(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/cart", strings.NewReader(`{"restaurant_id":"`+uuid.New().String()+`","item_id":"`+itemID.String()+`","quantity":1}`))
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated || !strings.Contains(resp.Body.String(), "cart.token.sig") {
		t.Fatalf("got %d %s", resp.Code, resp.Body.String())
	}
}
