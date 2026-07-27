package utils

import (
	"errors"
	"regexp"
	"strings"
)

var (
	indianMobileRegex = regexp.MustCompile(`^[6-9]\d{9}$`)
	otpRegex          = regexp.MustCompile(`^\d{6}$`)
)

// NormalizeE164 strips a country code or trunk prefix so downstream code
// always sees a bare 10-digit subscriber number.
//
// The bare "91"/"0" strips are gated on total length: a 10-digit Indian mobile
// number may itself start with "91" (e.g. 9123456789 is a real, valid number),
// so stripping it unconditionally corrupted every such number to 8 digits and
// failed it as invalid. Only strip when the extra 2 (or 1) digits are actually
// present — i.e. the input as a whole looks like <prefix><10-digit number>,
// not just a 10-digit number that happens to start the same way. "+91" stays
// unconditional since the '+' already makes it unambiguous.
func NormalizeE164(p string) string {
	phone := strings.TrimSpace(p)
	phone = strings.TrimPrefix(phone, "+91")
	if len(phone) == 12 {
		phone = strings.TrimPrefix(phone, "91")
	}
	if len(phone) == 11 {
		phone = strings.TrimPrefix(phone, "0")
	}
	return phone
}

func NormalizeIndianPhone(p string) string {
	return NormalizeE164(p)
}

func ValidateIndianPhone(p string) bool {
	phone := NormalizeIndianPhone(p)
	return indianMobileRegex.MatchString(phone)
}

func ValidateOTP(otp string) bool {
	return otpRegex.MatchString(strings.TrimSpace(otp))
}

func MaskPhone(phone string) string {
	normalized := NormalizeIndianPhone(phone)
	if len(normalized) <= 4 {
		return "XXXX"
	}
	return strings.Repeat("X", len(normalized)-4) + normalized[len(normalized)-4:]
}

func MaskEmail(email string) string {
	parts := strings.SplitN(strings.TrimSpace(email), "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "XXXX"
	}
	return "XXXX@" + parts[1]
}

func MaskKey(raw string) string {
	if raw == "" {
		return "XXXX"
	}
	if len(raw) <= 4 {
		return "XXXX"
	}
	return strings.Repeat("X", len(raw)-4) + raw[len(raw)-4:]
}

func RequireValidPhone(p string) (string, error) {
	normalized := NormalizeIndianPhone(p)
	if !ValidateIndianPhone(normalized) {
		return "", errors.New("must be a valid phone number")
	}
	return normalized, nil
}
