package otp

import (
	"context"

	"food-delivery-backend/pkg/utils"

	"github.com/rs/zerolog"
)

type MockProvider struct {
	log zerolog.Logger
}

func NewMockProvider(log zerolog.Logger) *MockProvider {
	return &MockProvider{log: log}
}

func (m *MockProvider) Send(_ context.Context, phone string, code string) error {
	m.log.Info().Str("phone", utils.MaskPhone(phone)).Str("otp", code).Msg("auth.otp.mock_sent")
	return nil
}

