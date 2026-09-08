package business

import (
	"context"
	"errors"
	"testing"

	"food-delivery-backend/internal/services/cart/models"
	"food-delivery-backend/internal/services/cart/repository"
	"food-delivery-backend/pkg/utils"
	"github.com/google/uuid"
)

type fakeRepository struct {
	addInput repository.AddItemInput
	addOut   repository.AddItemOutput
	getOut   models.Cart
	getID    uuid.UUID
}

func (f *fakeRepository) AddItem(_ context.Context, in repository.AddItemInput) (repository.AddItemOutput, error) {
	f.addInput = in
	return f.addOut, nil
}
func (f *fakeRepository) Get(_ context.Context, _ uuid.UUID, cartID uuid.UUID) (models.Cart, error) {
	f.getID = cartID
	return f.getOut, nil
}
func (f *fakeRepository) DeleteItem(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *fakeRepository) Clear(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func TestAddItemCreatesSignedTokenAndNormalizesCustomisations(t *testing.T) {
	repo := &fakeRepository{addOut: repository.AddItemOutput{CartID: uuid.New(), CartItemID: uuid.New()}}
	svc := NewService(repo, "test-secret")
	userID := uuid.New()
	restaurantID, itemID := uuid.New(), uuid.New()
	out, err := svc.AddItem(context.Background(), AddItemInput{UserID: userID.String(), DeviceID: "device-1", RestaurantID: restaurantID, ItemID: itemID, Quantity: 2, Customisations: []string{" Extra spicy "}})
	if err != nil {
		t.Fatal(err)
	}
	if out.CartToken == "" || repo.addInput.CartID != nil || len(repo.addInput.Customisations) != 1 || repo.addInput.Customisations[0] != "Extra spicy" {
		t.Fatalf("unexpected add input/output: %+v %+v", repo.addInput, out)
	}
	if _, err := svc.Get(context.Background(), GetInput{UserID: userID.String(), CartToken: out.CartToken}); err != nil {
		t.Fatal(err)
	}
	if repo.getID != repo.addOut.CartID {
		t.Fatalf("got cart %s, want %s", repo.getID, repo.addOut.CartID)
	}
}

func TestCartTokenRejectsTamperingAndWrongSecret(t *testing.T) {
	repo := &fakeRepository{addOut: repository.AddItemOutput{CartID: uuid.New(), CartItemID: uuid.New()}}
	svc := NewService(repo, "test-secret")
	token := "cart." + "not-a-real-id" + "." + utils.SignHMAC("not-a-real-id", "test-secret")
	if _, err := svc.Get(context.Background(), GetInput{UserID: uuid.New().String(), CartToken: token}); !errors.Is(err, ErrCartTokenInvalid) {
		t.Fatalf("got %v, want invalid token", err)
	}
	if _, err := svc.Get(context.Background(), GetInput{UserID: uuid.New().String(), CartToken: "cart.invalid.invalid"}); !errors.Is(err, ErrCartTokenInvalid) {
		t.Fatalf("got %v, want invalid token", err)
	}
}

func TestAddItemRejectsUnsafeInput(t *testing.T) {
	svc := NewService(&fakeRepository{}, "test-secret")
	base := AddItemInput{UserID: uuid.New().String(), RestaurantID: uuid.New(), ItemID: uuid.New(), Quantity: 1}
	for name, in := range map[string]AddItemInput{
		"missing user":     {RestaurantID: base.RestaurantID, ItemID: base.ItemID, Quantity: 1},
		"zero quantity":    {UserID: base.UserID, RestaurantID: base.RestaurantID, ItemID: base.ItemID},
		"large quantity":   {UserID: base.UserID, RestaurantID: base.RestaurantID, ItemID: base.ItemID, Quantity: 21},
		"too many options": {UserID: base.UserID, RestaurantID: base.RestaurantID, ItemID: base.ItemID, Quantity: 1, Customisations: make([]string, 9)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.AddItem(context.Background(), in); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
