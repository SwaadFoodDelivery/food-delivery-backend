package business

import (
	"context"
	"testing"
	"time"

	"food-delivery-backend/internal/services/payment/models"
	"github.com/google/uuid"
)

type fakeRepository struct {
	order   models.Order
	known   models.Payment
	found   bool
	created models.Payment
}

func (f *fakeRepository) FindOrderForPayment(context.Context, uuid.UUID, uuid.UUID) (models.Order, error) {
	return f.order, nil
}

func (f *fakeRepository) FindByIdempotency(context.Context, string) (models.Payment, bool, error) {
	return f.known, f.found, nil
}

func (f *fakeRepository) CreatePending(_ context.Context, payment models.Payment, _ string) (models.Payment, bool, error) {
	payment.PaymentID = uuid.New()
	payment.Status = models.StatusPending
	f.created = payment
	return payment, false, nil
}

func (f *fakeRepository) Complete(_ context.Context, paymentID uuid.UUID, status, providerPaymentID, failureCode, failureMessage string) (models.Payment, error) {
	f.created.PaymentID = paymentID
	f.created.Status = status
	f.created.ProviderPaymentID = providerPaymentID
	f.created.FailureCode = failureCode
	f.created.FailureMessage = failureMessage
	return f.created, nil
}

func TestPaySucceedsAndPersistsProviderReference(t *testing.T) {
	userID, orderID := uuid.New(), uuid.New()
	repo := &fakeRepository{order: models.Order{OrderID: orderID, UserID: userID, CreatedAt: time.Now(), TotalAmount: 43800, PaymentMethod: "upi", Status: "confirmed"}}
	svc := NewService(repo, NewMockProvider())
	out, replay, err := svc.Pay(context.Background(), Input{UserID: userID.String(), OrderID: orderID.String(), PaymentToken: "demo-token", IdempotencyKey: "payment-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if replay || out.Status != models.StatusSuccess || out.ProviderPaymentID == "" {
		t.Fatalf("got payment=%+v replay=%v", out, replay)
	}
}

func TestPayDeclinePersistsFailedOutcome(t *testing.T) {
	userID, orderID := uuid.New(), uuid.New()
	repo := &fakeRepository{order: models.Order{OrderID: orderID, UserID: userID, CreatedAt: time.Now(), TotalAmount: 43800, PaymentMethod: "card", Status: "confirmed"}}
	svc := NewService(repo, NewMockProvider())
	out, replay, err := svc.Pay(context.Background(), Input{UserID: userID.String(), OrderID: orderID.String(), PaymentToken: "mock_fail", IdempotencyKey: "payment-2"})
	serviceErr, ok := err.(*ServiceError)
	if !ok || serviceErr.StatusCode != 402 || serviceErr.Code != "PAYMENT_DECLINED" {
		t.Fatalf("got err=%v, want declined payment error", err)
	}
	if replay || out.Status != models.StatusFailed || out.FailureCode != "PAYMENT_DECLINED" {
		t.Fatalf("got payment=%+v replay=%v", out, replay)
	}
}

func TestPayReplaysCompletedPaymentBeforeReadingOrder(t *testing.T) {
	orderID := uuid.New()
	repo := &fakeRepository{found: true, known: models.Payment{OrderID: orderID, Status: models.StatusSuccess}}
	svc := NewService(repo, NewMockProvider())
	out, replay, err := svc.Pay(context.Background(), Input{UserID: uuid.New().String(), OrderID: orderID.String(), IdempotencyKey: "payment-3"})
	if err != nil || !replay || out.Status != models.StatusSuccess {
		t.Fatalf("got payment=%+v replay=%v err=%v", out, replay, err)
	}
}
