package storage

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const objectVerificationTimeout = 3 * time.Second

var objectVerificationClient = &http.Client{
	Timeout: objectVerificationTimeout,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func objectExists(ctx context.Context, objectURL string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, objectURL, nil)
	if err != nil {
		return false, fmt.Errorf("create object verification request: %w", err)
	}
	resp, err := objectVerificationClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("verify storage object: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return false, fmt.Errorf("verify storage object: unexpected HTTP status %d", resp.StatusCode)
	}
	return resp.ContentLength > 0, nil
}
