package otp

import "context"

type Provider interface {
	Send(ctx context.Context, phone string, code string) error
}

