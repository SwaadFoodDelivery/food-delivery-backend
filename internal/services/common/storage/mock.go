package storage

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"
)

type MockProvider struct {
	baseURL string
}

func NewMockProvider(baseURL string) *MockProvider {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = "http://localhost:9000"
	}
	return &MockProvider{baseURL: strings.TrimRight(base, "/")}
}

func (m *MockProvider) PresignPut(_ context.Context, in PresignPutInput) (*PresignPutOutput, error) {
	expiresAt := time.Now().UTC().Add(in.ExpiresIn)
	keyPath := path.Clean("/" + strings.TrimLeft(in.Bucket+"/"+in.Key, "/"))
	u := m.baseURL + keyPath
	if parsed, err := url.Parse(u); err == nil {
		q := parsed.Query()
		q.Set("mock_presigned", "true")
		q.Set("expires", expiresAt.Format(time.RFC3339))
		parsed.RawQuery = q.Encode()
		u = parsed.String()
	}
	return &PresignPutOutput{
		URL:       u,
		Method:    "PUT",
		Headers:   map[string]string{"Content-Type": in.ContentType},
		ExpiresAt: expiresAt,
	}, nil
}

func (m *MockProvider) ObjectExists(ctx context.Context, bucket, key string) (bool, error) {
	bucket, key = strings.TrimSpace(bucket), strings.TrimSpace(key)
	if bucket == "" || key == "" {
		return false, fmt.Errorf("bucket and key are required")
	}
	u, err := url.Parse(m.baseURL)
	if err != nil {
		return false, fmt.Errorf("invalid mock storage URL: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + path.Clean("/"+strings.TrimLeft(bucket+"/"+key, "/"))
	u.RawPath = ""
	return objectExists(ctx, u.String())
}
