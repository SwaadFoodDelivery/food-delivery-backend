package business

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/cart/models"
	"food-delivery-backend/internal/services/cart/repository"
	"food-delivery-backend/pkg/utils"

	"github.com/google/uuid"
)

var (
	ErrCartTokenInvalid   = &ServiceError{StatusCode: 401, Code: "CART_TOKEN_INVALID", Message: "cart token is invalid"}
	ErrRestaurantMismatch = &ServiceError{StatusCode: 409, Code: "RESTAURANT_MISMATCH", Message: "cart belongs to another restaurant"}
	ErrItemUnavailable    = &ServiceError{StatusCode: 409, Code: "ITEM_UNAVAILABLE", Message: "menu item is unavailable"}
	ErrItemNotFound       = &ServiceError{StatusCode: 404, Code: "ITEM_NOT_FOUND", Message: "menu item not found"}
	ErrCartItemNotFound   = &ServiceError{StatusCode: 404, Code: "CART_ITEM_NOT_FOUND", Message: "cart item not found"}
	ErrCartNotConfigured  = &ServiceError{StatusCode: 503, Code: "CART_NOT_CONFIGURED", Message: "cart signing is not configured"}
)

type ServiceError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *ServiceError) Error() string { return e.Message }

type Service interface {
	AddItem(context.Context, AddItemInput) (AddItemOutput, error)
	Get(context.Context, GetInput) (models.Cart, error)
	DeleteItem(context.Context, DeleteItemInput) error
	Clear(context.Context, ClearInput) error
}

type AddItemInput struct {
	UserID         string
	DeviceID       string
	CartToken      string
	RestaurantID   uuid.UUID
	ItemID         uuid.UUID
	Quantity       int
	Customisations []string
}

type AddItemOutput struct {
	CartToken  string
	CartItemID uuid.UUID
}

type GetInput struct {
	UserID    string
	CartToken string
}

type DeleteItemInput struct {
	UserID     string
	CartToken  string
	CartItemID uuid.UUID
}

type ClearInput struct {
	UserID    string
	CartToken string
}

type service struct {
	repo   repository.Repository
	secret string
}

func NewService(repo repository.Repository, secret string) Service {
	return &service{repo: repo, secret: strings.TrimSpace(secret)}
}

func (s *service) AddItem(ctx context.Context, in AddItemInput) (AddItemOutput, error) {
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return AddItemOutput{}, &ServiceError{StatusCode: 401, Code: apperrors.CodeInvalidToken, Message: "invalid user identity"}
	}
	if s.secret == "" {
		return AddItemOutput{}, ErrCartNotConfigured
	}
	if in.RestaurantID == uuid.Nil || in.ItemID == uuid.Nil {
		return AddItemOutput{}, &ServiceError{StatusCode: 400, Code: apperrors.CodeValidation, Message: "restaurant_id and item_id are required"}
	}
	if in.Quantity < 1 || in.Quantity > 20 {
		return AddItemOutput{}, &ServiceError{StatusCode: 400, Code: apperrors.CodeValidation, Message: "quantity must be between 1 and 20"}
	}
	customisations, err := normalizeCustomisations(in.Customisations)
	if err != nil {
		return AddItemOutput{}, &ServiceError{StatusCode: 400, Code: apperrors.CodeValidation, Message: err.Error()}
	}
	cartID, err := s.parseOptionalToken(in.CartToken)
	if err != nil {
		return AddItemOutput{}, ErrCartTokenInvalid
	}
	created, err := s.repo.AddItem(ctx, repository.AddItemInput{UserID: userID, DeviceID: strings.TrimSpace(in.DeviceID), CartID: cartID, RestaurantID: in.RestaurantID, ItemID: in.ItemID, Quantity: in.Quantity, Customisations: customisations})
	if err != nil {
		return AddItemOutput{}, mapRepositoryError(err)
	}
	return AddItemOutput{CartToken: s.token(created.CartID), CartItemID: created.CartItemID}, nil
}

func (s *service) Get(ctx context.Context, in GetInput) (models.Cart, error) {
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return models.Cart{}, &ServiceError{StatusCode: 401, Code: apperrors.CodeInvalidToken, Message: "invalid user identity"}
	}
	cartID, err := s.parseRequiredToken(in.CartToken)
	if err != nil {
		return models.Cart{}, ErrCartTokenInvalid
	}
	out, err := s.repo.Get(ctx, userID, cartID)
	if err != nil {
		return models.Cart{}, mapRepositoryError(err)
	}
	out.CartToken = s.token(cartID)
	return out, nil
}

func (s *service) DeleteItem(ctx context.Context, in DeleteItemInput) error {
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return &ServiceError{StatusCode: 401, Code: apperrors.CodeInvalidToken, Message: "invalid user identity"}
	}
	cartID, err := s.parseRequiredToken(in.CartToken)
	if err != nil || in.CartItemID == uuid.Nil {
		return ErrCartTokenInvalid
	}
	if err := s.repo.DeleteItem(ctx, userID, cartID, in.CartItemID); err != nil {
		return mapRepositoryError(err)
	}
	return nil
}

func (s *service) Clear(ctx context.Context, in ClearInput) error {
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return &ServiceError{StatusCode: 401, Code: apperrors.CodeInvalidToken, Message: "invalid user identity"}
	}
	cartID, err := s.parseRequiredToken(in.CartToken)
	if err != nil {
		return ErrCartTokenInvalid
	}
	if err := s.repo.Clear(ctx, userID, cartID); err != nil {
		return mapRepositoryError(err)
	}
	return nil
}

func (s *service) parseOptionalToken(raw string) (*uuid.UUID, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	id, err := s.parseRequiredToken(raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (s *service) parseRequiredToken(raw string) (uuid.UUID, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 || parts[0] != "cart" || s.secret == "" {
		return uuid.Nil, errors.New("invalid cart token")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return uuid.Nil, err
	}
	id, err := uuid.Parse(string(decoded))
	if err != nil || !utils.VerifyHMAC(id.String(), parts[2], s.secret) {
		return uuid.Nil, errors.New("invalid cart token")
	}
	return id, nil
}

func (s *service) token(id uuid.UUID) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(id.String()))
	return fmt.Sprintf("cart.%s.%s", encoded, utils.SignHMAC(id.String(), s.secret))
}

func normalizeCustomisations(values []string) ([]string, error) {
	if len(values) > 8 {
		return nil, errors.New("customisations cannot contain more than 8 values")
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 80 {
			return nil, errors.New("each customisation must be 1–80 characters")
		}
		out = append(out, value)
	}
	return out, nil
}

func mapRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrCartNotFound):
		return ErrCartTokenInvalid
	case errors.Is(err, repository.ErrRestaurantMismatch):
		return ErrRestaurantMismatch
	case errors.Is(err, repository.ErrItemUnavailable):
		return ErrItemUnavailable
	case errors.Is(err, repository.ErrItemNotFound):
		return ErrItemNotFound
	case errors.Is(err, repository.ErrCartItemNotFound):
		return ErrCartItemNotFound
	default:
		return err
	}
}
