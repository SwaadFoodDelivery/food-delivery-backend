package business

import (
	"context"
	"errors"
	"strings"
	"time"

	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/payment/models"
	"food-delivery-backend/internal/services/payment/repository"
	"github.com/google/uuid"
)

type Service interface {
	Pay(context.Context, Input) (models.Payment, bool, error)
}

type Input struct {
	UserID         string
	OrderID        string
	PaymentToken   string
	IdempotencyKey string
}

type ServiceError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *ServiceError) Error() string { return e.Message }

type service struct {
	repo     repository.Repository
	provider Provider
	delivery DeliveryService
}

type DeliveryService interface {
	EnsureForOrder(context.Context, uuid.UUID, time.Time) error
}

func NewService(repo repository.Repository, provider Provider, deliveries ...DeliveryService) Service {
	s := &service{repo: repo, provider: provider}
	if len(deliveries) > 0 {
		s.delivery = deliveries[0]
	}
	return s
}

func (s *service) schedulePaid(ctx context.Context, order models.Order, payment models.Payment) {
	if s.delivery != nil && payment.Status == models.StatusSuccess {
		// Payment is already durably successful. The delivery worker retries
		// missing assignments; a courier outage must not invite another charge.
		_ = s.delivery.EnsureForOrder(ctx, order.OrderID, order.CreatedAt)
	}
}

func (s *service) Pay(ctx context.Context, in Input) (models.Payment, bool, error) {
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return models.Payment{}, false, &ServiceError{StatusCode: 401, Code: apperrors.CodeInvalidToken, Message: "invalid user identity"}
	}
	orderID, err := uuid.Parse(strings.TrimSpace(in.OrderID))
	if err != nil {
		return models.Payment{}, false, &ServiceError{StatusCode: 400, Code: apperrors.CodeValidation, Message: "order_id must be a UUID"}
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" || len(key) > 64 {
		return models.Payment{}, false, &ServiceError{StatusCode: 400, Code: apperrors.CodeValidation, Message: "Idempotency-Key must be between 1 and 64 characters"}
	}
	order, err := s.repo.FindOrderForPayment(ctx, userID, orderID)
	if errors.Is(err, repository.ErrOrderNotFound) {
		return models.Payment{}, false, &ServiceError{StatusCode: 404, Code: "ORDER_NOT_FOUND", Message: "order not found"}
	}
	if err != nil {
		return models.Payment{}, false, err
	}
	if order.PaymentMethod == "cash_on_delivery" {
		return models.Payment{}, false, &ServiceError{StatusCode: 409, Code: "PAYMENT_NOT_REQUIRED", Message: "cash on delivery does not require a payment attempt"}
	}
	if order.Status == "cancelled" || order.Status == "rejected" || order.Status == "delivered" {
		return models.Payment{}, false, &ServiceError{StatusCode: 409, Code: "ORDER_NOT_PAYABLE", Message: "cancelled, rejected, or delivered orders cannot be paid"}
	}
	if existing, found, err := s.repo.FindByIdempotency(ctx, key); err != nil {
		return models.Payment{}, false, err
	} else if found {
		if existing.OrderID != orderID {
			return models.Payment{}, false, &ServiceError{StatusCode: 409, Code: "IDEMPOTENCY_CONFLICT", Message: "payment idempotency key belongs to another order"}
		}
		if existing.Status == models.StatusFailed {
			return existing, true, &ServiceError{StatusCode: 402, Code: existing.FailureCode, Message: "payment was declined"}
		}
		if existing.Status == models.StatusSuccess {
			s.schedulePaid(ctx, order, existing)
			return existing, true, nil
		}
		// Pending attempts pass through the repository's serialized mock expiry
		// policy, including retries using the original key after an interruption.
	}
	pending, replay, err := s.repo.CreatePending(ctx, models.Payment{OrderID: order.OrderID, CreatedAt: order.CreatedAt, UserID: userID, Amount: order.TotalAmount, Provider: "mock"}, key)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotPayable) {
			return models.Payment{}, false, &ServiceError{StatusCode: 409, Code: "ORDER_NOT_PAYABLE", Message: "order is no longer payable"}
		}
		return models.Payment{}, false, err
	}
	if replay {
		if pending.OrderID != orderID {
			return models.Payment{}, false, &ServiceError{StatusCode: 409, Code: "IDEMPOTENCY_CONFLICT", Message: "payment idempotency key belongs to another order"}
		}
		if pending.Status == models.StatusPending {
			return models.Payment{}, false, &ServiceError{StatusCode: 409, Code: "PAYMENT_IN_PROGRESS", Message: "payment is in progress; retry after one minute or cancel the saved demo order"}
		}
		if pending.Status == models.StatusFailed {
			return pending, true, &ServiceError{StatusCode: 402, Code: pending.FailureCode, Message: "payment was declined"}
		}
		s.schedulePaid(ctx, order, pending)
		return pending, true, nil
	}
	charge, chargeErr := s.provider.Charge(ctx, ChargeInput{AmountMinor: order.TotalAmount, Currency: "INR", PaymentToken: in.PaymentToken})
	if chargeErr != nil {
		failureCode, message := "PAYMENT_FAILED", "payment was declined"
		if errors.Is(chargeErr, ErrDeclined) {
			failureCode, message = "PAYMENT_DECLINED", "mock payment was declined"
		}
		failed, err := s.repo.Complete(ctx, pending.PaymentID, models.StatusFailed, "", failureCode, message)
		if err != nil {
			return models.Payment{}, false, err
		}
		return failed, false, &ServiceError{StatusCode: 402, Code: failureCode, Message: message}
	}
	completed, err := s.repo.Complete(ctx, pending.PaymentID, models.StatusSuccess, charge.ProviderPaymentID, "", "")
	if err != nil {
		return models.Payment{}, false, err
	}
	if completed.Status != models.StatusSuccess {
		return completed, false, &ServiceError{StatusCode: 402, Code: completed.FailureCode, Message: "mock payment was interrupted; please retry"}
	}
	s.schedulePaid(ctx, order, completed)
	return completed, false, nil
}
