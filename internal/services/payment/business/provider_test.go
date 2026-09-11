package business

import (
	"context"
	"testing"
)

func TestMockProviderChargesAndDeclinesSentinel(t *testing.T) {
	provider := NewMockProvider()
	charge, err := provider.Charge(context.Background(), ChargeInput{AmountMinor: 12500, Currency: "INR", PaymentToken: "demo-token"})
	if err != nil {
		t.Fatalf("unexpected success error: %v", err)
	}
	if charge.ProviderPaymentID == "" {
		t.Fatal("expected mock provider payment id")
	}
	if _, err := provider.Charge(context.Background(), ChargeInput{AmountMinor: 12500, Currency: "INR", PaymentToken: "mock_fail"}); err != ErrDeclined {
		t.Fatalf("got %v, want ErrDeclined", err)
	}
}
