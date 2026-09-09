package business

import (
	"context"
	"errors"
	"strings"
	"time"

	apperrors "food-delivery-backend/internal/errors"
	cartbusiness "food-delivery-backend/internal/services/cart/business"
	ordermodels "food-delivery-backend/internal/services/order/models"
	"food-delivery-backend/internal/services/order/repository"

	"github.com/google/uuid"
)

type Service interface {
	Quote(context.Context, QuoteInput) (ordermodels.Quote, error)
	Place(context.Context, PlaceInput) (ordermodels.Order, bool, error)
	List(context.Context, uuid.UUID, int) ([]ordermodels.HistoryItem, error)
	History(context.Context, uuid.UUID, uuid.UUID) (ordermodels.History, error)
}

type DeliveryService interface {
	EnsureForOrder(context.Context, uuid.UUID, time.Time) error
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
	carts    cartbusiness.Service
	repo     repository.Repository
	delivery DeliveryService
}

func NewService(carts cartbusiness.Service, repo repository.Repository, deliveries ...DeliveryService) Service {
	var delivery DeliveryService
	if len(deliveries) > 0 {
		delivery = deliveries[0]
	}
	return &service{carts: carts, repo: repo, delivery: delivery}
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
		if err := s.ensureDelivery(ctx, existing); err != nil {
			return ordermodels.Order{}, false, err
		}
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
	if err := s.ensureDelivery(ctx, out); err != nil {
		return ordermodels.Order{}, false, err
	}
	return out, replay, nil
}

func (s *service) List(ctx context.Context, userID uuid.UUID, limit int) ([]ordermodels.HistoryItem, error) {
	historyRepo, ok := s.repo.(repository.HistoryRepository)
	if !ok {
		return nil, &ServiceError{StatusCode: 500, Code: "ORDER_HISTORY_UNAVAILABLE", Message: "order history is unavailable"}
	}
	if limit < 1 || limit > 50 {
		return nil, &ServiceError{StatusCode: 400, Code: apperrors.CodeValidation, Message: "limit must be between 1 and 50"}
	}
	return historyRepo.ListForUser(ctx, userID, limit)
}

func (s *service) History(ctx context.Context, userID, orderID uuid.UUID) (ordermodels.History, error) {
	historyRepo, ok := s.repo.(repository.HistoryRepository)
	if !ok {
		return ordermodels.History{}, &ServiceError{StatusCode: 500, Code: "ORDER_HISTORY_UNAVAILABLE", Message: "order history is unavailable"}
	}
	return historyRepo.GetHistory(ctx, userID, orderID)
}

func (s *service) ensureDelivery(ctx context.Context, order ordermodels.Order) error {
	if s.delivery == nil {
		return nil
	}
	if err := s.delivery.EnsureForOrder(ctx, order.OrderID, order.CreatedAt); err != nil {
		return &ServiceError{StatusCode: 503, Code: "DELIVERY_UNAVAILABLE", Message: "mock delivery partner is unavailable"}
	}
	return nil
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
