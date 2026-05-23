package utils

import "testing"

func TestMaskPhone(t *testing.T) {
	if got := MaskPhone("9876543210"); got != "XXXXXX3210" {
		t.Fatalf("unexpected mask: %s", got)
	}
	if got := MaskPhone("+919876543210"); got != "XXXXXX3210" {
		t.Fatalf("unexpected mask with country code: %s", got)
	}
}

func TestMaskEmail(t *testing.T) {
	if got := MaskEmail("user@example.com"); got != "XXXX@example.com" {
		t.Fatalf("unexpected email mask: %s", got)
	}
}

func TestValidateIndianPhone(t *testing.T) {
	valid := []string{"9876543210", "+919876543210", "09876543210"}
	for _, v := range valid {
		if !ValidateIndianPhone(v) {
			t.Fatalf("expected phone valid: %s", v)
		}
	}

	invalid := []string{"5876543210", "12345", "abc1234567", "98765432101"}
	for _, v := range invalid {
		if ValidateIndianPhone(v) {
			t.Fatalf("expected phone invalid: %s", v)
		}
	}
}

func TestValidateOTP(t *testing.T) {
	if !ValidateOTP("439954") {
		t.Fatalf("expected otp valid")
	}
	invalid := []string{"12345", "abcdef", "1234567"}
	for _, v := range invalid {
		if ValidateOTP(v) {
			t.Fatalf("expected otp invalid: %s", v)
		}
	}
}
