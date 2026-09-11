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
	// ObjectExists verifies that the object exists and has a positive content length.
	// Missing, empty, or unknown-length objects return false; verification failures return an error.
	ObjectExists(ctx context.Context, bucket, key string) (bool, error)
}
