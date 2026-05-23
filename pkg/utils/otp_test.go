package utils

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestGenerateOTP(t *testing.T) {
	otp, err := GenerateOTP()
	if err != nil {
		t.Fatalf("GenerateOTP returned error: %v", err)
	}
	if len(otp) != 6 {
		t.Fatalf("expected 6-digit otp, got %s", otp)
	}
	for _, ch := range otp {
		if ch < '0' || ch > '9' {
			t.Fatalf("otp must be numeric: %s", otp)
		}
	}
}

func TestHashAndCompareOTP(t *testing.T) {
	otp := "123456"
	hash, err := HashOTP(otp)
	if err != nil {
		t.Fatalf("HashOTP returned error: %v", err)
	}
	if strings.Contains(hash, otp) {
		t.Fatalf("hash should not contain plaintext otp")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(otp)) != nil {
		t.Fatalf("expected hash compare to succeed")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("999999")) == nil {
		t.Fatalf("expected hash compare to fail for wrong otp")
	}
}
