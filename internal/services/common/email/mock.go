package email

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

func (m *MockProvider) SendVerificationOTP(ctx context.Context, to string, code string) error {
	ctxLog := zerolog.Ctx(ctx)
	if ctxLog == nil || ctxLog.GetLevel() == zerolog.Disabled {
		ctxLog = &m.log
	}
	ctxLog.Info().Str("email", utils.MaskEmail(to)).Str("otp", code).Msg("auth.email_otp.mock_sent")
	return nil
}
