package email

import "context"

type Provider interface {
	SendVerificationOTP(ctx context.Context, to string, code string) error
}
