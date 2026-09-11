package business

import (
	"context"
	"errors"
	"testing"
	"time"

	"food-delivery-backend/internal/services/payment/models"
	"github.com/google/uuid"
)

type recordingDelivery struct {
	calls int
	err   error
}

func (d *recordingDelivery) EnsureForOrder(context.Context, uuid.UUID, time.Time) error {
	d.calls++
	return d.err
}

func TestPaymentOnlySchedulesAfterPersistedSuccess(t *testing.T) {
	for _, token := range []string{"demo-token", "mock_fail"} {
		t.Run(token, func(t *testing.T) {
			userID, orderID := uuid.New(), uuid.New()
			r := &fakeRepository{order: models.Order{OrderID: orderID, UserID: userID, CreatedAt: time.Now(), TotalAmount: 10000, PaymentMethod: "upi", Status: "order_created"}}
			d := &recordingDelivery{err: errors.New("no courier")}
			s := NewService(r, NewMockProvider(), d)
			out, _, err := s.Pay(context.Background(), Input{UserID: userID.String(), OrderID: orderID.String(), PaymentToken: token, IdempotencyKey: "schedule-test"})
			if token == "mock_fail" {
				if err == nil || d.calls != 0 {
					t.Fatalf("declined payment scheduled delivery: calls=%d err=%v", d.calls, err)
				}
				return
			}
			if err != nil || out.Status != models.StatusSuccess || d.calls != 1 {
				t.Fatalf("successful payment lost during courier outage: %+v calls=%d err=%v", out, d.calls, err)
			}
			r.known = out
			r.found = true
			_, replay, err := s.Pay(context.Background(), Input{UserID: userID.String(), OrderID: orderID.String(), IdempotencyKey: "schedule-test"})
			if err != nil || !replay || d.calls != 2 {
				t.Fatalf("success replay did not retry assignment: %v %v calls=%d", replay, err, d.calls)
			}
		})
	}
}

type fakeRepository struct {
	order   models.Order
	known   models.Payment
	found   bool
	created models.Payment
}

func TestDeclinedReplayIsNotReportedAsSuccess(t *testing.T) {
	id := uuid.New()
	r := &fakeRepository{found: true, known: models.Payment{OrderID: id, Status: models.StatusFailed, FailureCode: "PAYMENT_DECLINED"}}
	out, replay, err := NewService(r, NewMockProvider()).Pay(context.Background(), Input{UserID: uuid.NewString(), OrderID: id.String(), IdempotencyKey: "declined"})
	var se *ServiceError
	if !errors.As(err, &se) || se.StatusCode != 402 || !replay || out.Status != models.StatusFailed {
		t.Fatalf("declined replay=%v out=%+v err=%v", replay, out, err)
	}
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
