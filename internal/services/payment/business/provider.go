package business

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

var ErrDeclined = errors.New("mock payment declined")

type ChargeInput struct {
	AmountMinor  int64
	Currency     string
	PaymentToken string
}

type ChargeOutput struct {
	ProviderPaymentID string
}

type Provider interface {
	Charge(context.Context, ChargeInput) (ChargeOutput, error)
}

type MockProvider struct{}

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (p *MockProvider) Charge(_ context.Context, in ChargeInput) (ChargeOutput, error) {
	if in.AmountMinor <= 0 {
		return ChargeOutput{}, errors.New("payment amount must be positive")
	}
	if strings.EqualFold(strings.TrimSpace(in.PaymentToken), "mock_fail") || strings.EqualFold(strings.TrimSpace(in.PaymentToken), "decline") {
		return ChargeOutput{}, ErrDeclined
	}
	return ChargeOutput{ProviderPaymentID: "mock_pay_" + uuid.NewString()}, nil
}
