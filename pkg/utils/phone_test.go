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

// A bare 10-digit number that happens to start with "91" (a real, allocated
// range — e.g. 9123456789) must NOT be mistaken for <country-code>+<8 digits>.
// Regression test for a bug where NormalizeE164 stripped "91" unconditionally.
func TestNormalizeE164DoesNotMangleNumbersStartingWith91(t *testing.T) {
	cases := map[string]string{
		"9123456789":    "9123456789", // 10-digit local number starting with 91
		"919123456789":  "9123456789", // country code + that same number
		"+919123456789": "9123456789", // with a leading plus
		"09123456789":   "9123456789", // trunk-prefixed
	}
	for input, want := range cases {
		if got := NormalizeE164(input); got != want {
			t.Fatalf("NormalizeE164(%q) = %q, want %q", input, got, want)
		}
	}
	if !ValidateIndianPhone("9123456789") {
		t.Fatalf("expected 9123456789 to be a valid phone number")
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
