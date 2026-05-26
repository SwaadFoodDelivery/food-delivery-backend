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

func (m *MockProvider) Send(ctx context.Context, phone string, code string) error {
	ctxLog := zerolog.Ctx(ctx)
	if ctxLog == nil || ctxLog.GetLevel() == zerolog.Disabled {
		ctxLog = &m.log
	}
	ctxLog.Info().Str("phone", utils.MaskPhone(phone)).Str("otp", code).Msg("auth.otp.mock_sent")
	return nil
}
