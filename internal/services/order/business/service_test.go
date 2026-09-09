package business

import (
	"context"
	"errors"
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
	err      error
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
func (f fakeOrderRepository) CheckServiceability(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (ordermodels.Serviceability, error) {
	return ordermodels.Serviceability{}, f.err
}

func TestServiceabilityMapsRestaurantNotFound(t *testing.T) {
	svc := NewService(&fakeCartService{}, fakeOrderRepository{err: repository.ErrRestaurantNotFound})
	_, err := svc.Serviceability(context.Background(), ServiceabilityInput{
		UserID: uuid.New().String(), RestaurantID: uuid.New().String(), AddressID: uuid.New().String(),
	})
	serviceErr, ok := err.(*ServiceError)
	if !ok || serviceErr.StatusCode != 404 || serviceErr.Code != "RESTAURANT_NOT_FOUND" {
		t.Fatalf("got %#v, want restaurant-not-found 404", err)
	}
}

func TestServiceabilityPreservesUnexpectedRepositoryError(t *testing.T) {
	want := errors.New("database unavailable")
	svc := NewService(&fakeCartService{}, fakeOrderRepository{err: want})
	_, err := svc.Serviceability(context.Background(), ServiceabilityInput{
		UserID: uuid.New().String(), RestaurantID: uuid.New().String(), AddressID: uuid.New().String(),
	})
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
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

func TestPlaceRejectsOversizedIdempotencyKey(t *testing.T) {
	svc := NewService(&fakeCartService{}, fakeOrderRepository{})
	_, _, err := svc.Place(context.Background(), PlaceInput{UserID: uuid.New().String(), AddressID: uuid.New().String(), PaymentMethod: "upi", IdempotencyKey: "12345678901234567890123456789012345678901234567890123456789012345"})
	serviceErr, ok := err.(*ServiceError)
	if !ok || serviceErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("got %#v, want validation error", err)
	}
}

var _ repository.Repository = fakeOrderRepository{}
