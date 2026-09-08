package business

import (
	"context"
	"errors"
	"strings"

	apperrors "food-delivery-backend/internal/errors"
	cartbusiness "food-delivery-backend/internal/services/cart/business"
	ordermodels "food-delivery-backend/internal/services/order/models"
	"food-delivery-backend/internal/services/order/repository"

	"github.com/google/uuid"
)

type Service interface {
	Quote(context.Context, QuoteInput) (ordermodels.Quote, error)
	Place(context.Context, PlaceInput) (ordermodels.Order, bool, error)
}

type QuoteInput struct {
	UserID    string
	CartToken string
	AddressID string
}

type PlaceInput struct {
	UserID         string
	CartToken      string
	AddressID      string
	PaymentMethod  string
	Instructions   string
	IdempotencyKey string
}

type ServiceError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *ServiceError) Error() string { return e.Message }

type service struct {
	carts cartbusiness.Service
	repo  repository.Repository
}

func NewService(carts cartbusiness.Service, repo repository.Repository) Service {
	return &service{carts: carts, repo: repo}
}

func (s *service) Quote(ctx context.Context, in QuoteInput) (ordermodels.Quote, error) {
	userID, addressID, err := parseIDs(in.UserID, in.AddressID)
	if err != nil {
		return ordermodels.Quote{}, err
	}
	cart, err := s.carts.Get(ctx, cartbusiness.GetInput{UserID: userID.String(), CartToken: strings.TrimSpace(in.CartToken)})
	if err != nil {
		return ordermodels.Quote{}, err
	}
	out, err := s.repo.Quote(ctx, userID, cart, addressID)
	if err != nil {
		return ordermodels.Quote{}, mapRepositoryError(err)
	}
	return out, nil
}

func (s *service) Place(ctx context.Context, in PlaceInput) (ordermodels.Order, bool, error) {
	userID, addressID, err := parseIDs(in.UserID, in.AddressID)
	if err != nil {
		return ordermodels.Order{}, false, err
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" || len(key) > 64 {
		return ordermodels.Order{}, false, &ServiceError{StatusCode: 400, Code: apperrors.CodeValidation, Message: "Idempotency-Key must be between 1 and 64 characters"}
	}
	method := strings.ToLower(strings.TrimSpace(in.PaymentMethod))
	if method != "upi" && method != "card" && method != "cash_on_delivery" {
		return ordermodels.Order{}, false, &ServiceError{StatusCode: 400, Code: apperrors.CodeValidation, Message: "payment_method must be upi, card, or cash_on_delivery"}
	}
	if existing, replay, err := s.repo.FindByIdempotency(ctx, userID, key); err != nil {
		return ordermodels.Order{}, false, err
	} else if replay {
		return existing, true, nil
	}
	cart, err := s.carts.Get(ctx, cartbusiness.GetInput{UserID: userID.String(), CartToken: strings.TrimSpace(in.CartToken)})
	if err != nil {
		return ordermodels.Order{}, false, err
	}
	quote, err := s.repo.Quote(ctx, userID, cart, addressID)
	if err != nil {
		return ordermodels.Order{}, false, mapRepositoryError(err)
	}
	out, replay, err := s.repo.Place(ctx, ordermodels.PlaceInput{UserID: userID, CartToken: strings.TrimSpace(in.CartToken), AddressID: addressID, PaymentMethod: method, Instructions: strings.TrimSpace(in.Instructions), IdempotencyKey: key}, cart, quote)
	if err != nil {
		return ordermodels.Order{}, false, mapRepositoryError(err)
	}
	return out, replay, nil
}

func parseIDs(userIDRaw, addressIDRaw string) (uuid.UUID, uuid.UUID, error) {
	userID, err := uuid.Parse(strings.TrimSpace(userIDRaw))
	if err != nil {
		return uuid.Nil, uuid.Nil, &ServiceError{StatusCode: 401, Code: apperrors.CodeInvalidToken, Message: "invalid user identity"}
	}
	addressID, err := uuid.Parse(strings.TrimSpace(addressIDRaw))
	if err != nil {
		return uuid.Nil, uuid.Nil, &ServiceError{StatusCode: 400, Code: apperrors.CodeValidation, Message: "address_id must be a UUID"}
	}
	return userID, addressID, nil
}

func mapRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrCartEmpty):
		return &ServiceError{StatusCode: 400, Code: "CART_EMPTY", Message: "cart is empty"}
	case errors.Is(err, repository.ErrCartNotActive):
		return &ServiceError{StatusCode: 409, Code: "CART_NOT_ACTIVE", Message: "cart has already been converted or expired"}
	case errors.Is(err, repository.ErrItemPriceChanged):
		return &ServiceError{StatusCode: 409, Code: "ITEM_PRICE_CHANGED", Message: "a menu item price changed; review your cart"}
	case errors.Is(err, repository.ErrItemUnavailable):
		return &ServiceError{StatusCode: 409, Code: "ITEM_UNAVAILABLE", Message: "a menu item is unavailable"}
	case errors.Is(err, repository.ErrAddressNotFound):
		return &ServiceError{StatusCode: 404, Code: "ADDRESS_NOT_FOUND", Message: "address not found"}
	case errors.Is(err, repository.ErrNotServiceable):
		return &ServiceError{StatusCode: 422, Code: "ADDRESS_NOT_SERVICEABLE", Message: "address is outside the restaurant service area"}
	case errors.Is(err, repository.ErrInvalidPayment):
		return &ServiceError{StatusCode: 400, Code: apperrors.CodeValidation, Message: "payment_method is invalid"}
	default:
		return err
	}
}
