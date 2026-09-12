package otp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func TestPrivateMockOutbox(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	provider, err := NewMockProviderWithOutbox(zerolog.New(&logs), dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.Send(context.Background(), "9000000999", "654321"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%d err=%v", len(entries), err)
	}
	path := filepath.Join(dir, entries[0].Name())
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("delivery permissions are not private")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var message struct {
		Code      string `json:"code"`
		PhoneHash string `json:"phone_hash"`
	}
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatal(err)
	}
	if message.Code != "654321" || len(message.PhoneHash) != 64 || bytes.Contains(data, []byte("9000000999")) || logs.Len() != 0 {
		t.Fatal("incorrect or leaking outbox delivery")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if provider.Send(ctx, "9000000999", "111111") == nil {
		t.Fatal("cancelled send succeeded")
	}
	for _, env := range []string{"production", "staging", ""} {
		if _, err := NewMockProviderWithOutbox(zerolog.Nop(), dir, env); err == nil {
			t.Fatalf("allowed %s outbox", env)
		}
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMockProviderWithOutbox(zerolog.Nop(), dir, "test"); err == nil {
		t.Fatal("allowed public directory")
	}
	if _, err := NewMockProviderWithOutbox(zerolog.Nop(), "relative", "test"); err == nil {
		t.Fatal("allowed relative directory")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMockProviderWithOutbox(zerolog.Nop(), link, "test"); err == nil {
		t.Fatal("allowed symlink directory")
	}
}
