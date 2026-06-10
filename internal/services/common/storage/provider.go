package storage

import (
	"context"
	"time"
)

type PresignPutInput struct {
	Bucket      string
	Key         string
	ContentType string
	ExpiresIn   time.Duration
}

type PresignPutOutput struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	ExpiresAt time.Time         `json:"expires_at"`
}

type Provider interface {
	PresignPut(ctx context.Context, in PresignPutInput) (*PresignPutOutput, error)
}
