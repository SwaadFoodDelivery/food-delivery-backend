package business

import (
	"context"
	"testing"

	cartbusiness "food-delivery-backend/internal/services/cart/business"
	cartmodels "food-delivery-backend/internal/services/cart/models"
	ordermodels "food-delivery-backend/internal/services/order/models"
	"food-delivery-backend/internal/services/order/repository"
	"github.com/google/uuid"
)

type fakeCartService struct{ gets int }

func (f *fakeCartService) AddItem(context.Context, cartbusiness.AddItemInput) (cartbusiness.AddItemOutput, error) {
	return cartbusiness.AddItemOutput{}, nil
}
func (f *fakeCartService) Get(context.Context, cartbusiness.GetInput) (cartmodels.Cart, error) {
	f.gets++
	return cartmodels.Cart{}, nil
}
func (f *fakeCartService) DeleteItem(context.Context, cartbusiness.DeleteItemInput) error { return nil }
func (f *fakeCartService) Clear(context.Context, cartbusiness.ClearInput) error           { return nil }

type fakeOrderRepository struct {
	existing ordermodels.Order
	found    bool
}

func (f fakeOrderRepository) Quote(context.Context, uuid.UUID, cartmodels.Cart, uuid.UUID) (ordermodels.Quote, error) {
	return ordermodels.Quote{}, nil
}
func (f fakeOrderRepository) FindByIdempotency(context.Context, uuid.UUID, string) (ordermodels.Order, bool, error) {
	return f.existing, f.found, nil
}
func (f fakeOrderRepository) Place(context.Context, ordermodels.PlaceInput, cartmodels.Cart, ordermodels.Quote) (ordermodels.Order, bool, error) {
	return ordermodels.Order{}, false, nil
}

func TestPlaceReplaysBeforeReadingConvertedCart(t *testing.T) {
	carts := &fakeCartService{}
	existing := ordermodels.Order{OrderID: uuid.New(), Status: "order_created"}
	svc := NewService(carts, fakeOrderRepository{existing: existing, found: true})
	out, replay, err := svc.Place(context.Background(), PlaceInput{
		UserID: uuid.New().String(), AddressID: uuid.New().String(),
		PaymentMethod: "upi", IdempotencyKey: uuid.New().String(), CartToken: "converted-cart-token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !replay || out.OrderID != existing.OrderID {
		t.Fatalf("got replay=%v order=%s, want existing order", replay, out.OrderID)
	}
	if carts.gets != 0 {
		t.Fatalf("cart was read %d times during replay", carts.gets)
	}
}

func TestPlaceRejectsInvalidIdempotencyKey(t *testing.T) {
	svc := NewService(&fakeCartService{}, fakeOrderRepository{})
	_, _, err := svc.Place(context.Background(), PlaceInput{UserID: uuid.New().String(), AddressID: uuid.New().String(), PaymentMethod: "upi", IdempotencyKey: "request-1"})
	serviceErr, ok := err.(*ServiceError)
	if !ok || serviceErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("got %#v, want validation error", err)
	}
}

var _ repository.Repository = fakeOrderRepository{}
