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

func NormalizeE164(p string) string {
	phone := strings.TrimSpace(p)
	phone = strings.TrimPrefix(phone, "+91")
	phone = strings.TrimPrefix(phone, "91")
	phone = strings.TrimPrefix(phone, "0")
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
