package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMinIOPutHeadIntegration is opt-in; see README.md for its environment.
// It uses the real provider and HTTP transport, with no signature test doubles.
func TestMinIOPutHeadIntegration(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("STORAGE_MINIO_ENDPOINT"))
	if endpoint == "" {
		t.Skip("set STORAGE_MINIO_ENDPOINT to run against an actual MinIO server")
	}
	required := func(name string) string {
		t.Helper()
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			t.Fatalf("%s is required when STORAGE_MINIO_ENDPOINT is set", name)
		}
		return value
	}
	bucket := required("STORAGE_MINIO_BUCKET")
	accessKey := required("STORAGE_MINIO_ACCESS_KEY")
	secretKey := required("STORAGE_MINIO_SECRET_KEY")
	region := strings.TrimSpace(os.Getenv("STORAGE_MINIO_REGION"))
	if region == "" {
		region = "us-east-1"
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		t.Fatal("STORAGE_MINIO_ENDPOINT must be an HTTP(S) endpoint without credentials, query, or fragment")
	}
	if strings.ContainsAny(bucket, "/\\?#") || bucket == "." || bucket == ".." {
		t.Fatal("STORAGE_MINIO_BUCKET must be a bucket name, not a path")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal("could not generate a unique integration test key")
	}
	key := "storage-integration/" + hex.EncodeToString(nonce[:]) + "/signed upload.txt"
	// Both endpoints intentionally reach the real server: PUT uses the public
	// signing URL while ObjectExists signs HEAD against the internal endpoint.
	provider := NewDevProvider(accessKey, secretKey, region, endpoint, endpoint)
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	t.Cleanup(client.CloseIdleConnections)
	checkExists := func(ctx context.Context, phase string, want bool) {
		t.Helper()
		exists, err := provider.ObjectExists(ctx, bucket, key)
		if err != nil {
			// HTTP errors can contain signed URLs. Never print the raw error.
			t.Fatalf("%s: ObjectExists failed (error type %T; details omitted to protect signed URLs)", phase, err)
		}
		if exists != want {
			t.Fatalf("%s: ObjectExists = %v, want %v", phase, exists, want)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	checkExists(ctx, "before PUT", false)

	// Delete only this randomly generated key. Register before PUT so cleanup
	// still runs if MinIO stores the upload but the client loses the response.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		signed, err := provider.presign(http.MethodDelete, endpoint, PresignPutInput{
			Bucket: bucket, Key: key, ExpiresIn: time.Minute,
		})
		if err != nil {
			t.Errorf("cleanup signing failed; fixture remains at bucket=%s key=%s", bucket, key)
			return
		}
		req, err := http.NewRequestWithContext(cleanupCtx, http.MethodDelete, signed.URL, nil)
		if err != nil {
			t.Errorf("cleanup request failed; fixture remains at bucket=%s key=%s", bucket, key)
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("cleanup transport failed; fixture may remain at bucket=%s key=%s", bucket, key)
			return
		}
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Errorf("cleanup returned HTTP %d; fixture may remain at bucket=%s key=%s", resp.StatusCode, bucket, key)
			return
		}
		// Require an actual 404: ObjectExists(false) alone also accepts an
		// empty object, which is the last upload in this test.
		head, err := provider.presign(http.MethodHead, endpoint, PresignPutInput{
			Bucket: bucket, Key: key, ExpiresIn: time.Minute,
		})
		if err != nil {
			t.Error("cleanup verification signing failed")
			return
		}
		req, err = http.NewRequestWithContext(cleanupCtx, http.MethodHead, head.URL, nil)
		if err != nil {
			t.Error("cleanup verification request creation failed")
			return
		}
		resp, err = client.Do(req)
		if err != nil {
			t.Errorf("cleanup verification failed; check bucket=%s key=%s", bucket, key)
			return
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("cleanup HEAD returned HTTP %d, want 404; check bucket=%s key=%s", resp.StatusCode, bucket, key)
			return
		}
		t.Log("deleted the generated integration object; signed HEAD confirms it is absent")
	})

	for _, payload := range []string{"verified MinIO signed upload\n", ""} {
		signed, err := provider.PresignPut(ctx, PresignPutInput{
			Bucket: bucket, Key: key, ContentType: "text/plain", ExpiresIn: time.Minute,
		})
		if err != nil {
			t.Fatal("PUT signing failed; details omitted to protect credentials")
		}
		if signed.Method != http.MethodPut {
			t.Fatal("PresignPut returned an unexpected method")
		}
		req, err := http.NewRequestWithContext(ctx, signed.Method, signed.URL, strings.NewReader(payload))
		if err != nil {
			t.Fatal("PUT request creation failed; details omitted to protect signed URL")
		}
		for name, value := range signed.Headers {
			req.Header.Set(name, value)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("PUT transport failed (error type %T; details omitted to protect signed URL)", err)
		}
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Fatalf("signed PUT returned HTTP %d", resp.StatusCode)
		}
		checkExists(ctx, "after PUT", len(payload) > 0)
		t.Logf("real MinIO accepted signed PUT (%d bytes) and UNSIGNED-PAYLOAD signed HEAD; ObjectExists=%v", len(payload), len(payload) > 0)
	}
}
