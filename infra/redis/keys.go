package redis

import (
	"fmt"

	"food-delivery-backend/internal/constants"
)

func SessionKey(sessionID string) string {
	return fmt.Sprintf(constants.RedisSessionKeyPattern, sessionID)
}

func CartKey(userID string) string {
	return fmt.Sprintf(constants.RedisCartKeyPattern, userID)
}

// OTPKey and OTPRateKey are scoped by role as well as phone: one phone number
// may hold a separate account per role, and each of those accounts gets its own
// OTP and its own resend budget.
func OTPKey(phone, role string) string {
	return fmt.Sprintf(constants.RedisOTPKeyPattern, phone, role)
}

func OTPRateKey(phone, role string) string {
	return fmt.Sprintf(constants.RedisOTPRateLimitKeyPattern, phone, role)
}

func EmailOTPKey(userID, email string) string {
	return fmt.Sprintf(constants.RedisEmailOTPKeyPattern, userID, email)
}

func EmailOTPAttemptsKey(userID, email string) string {
	return fmt.Sprintf(constants.RedisEmailOTPAttemptsKeyPattern, userID, email)
}

func EmailOTPRateKey(userID, email string) string {
	return fmt.Sprintf(constants.RedisEmailOTPRateLimitKeyPattern, userID, email)
}

// EmailVerifiedKey marks an email as verified for a given guest session, which is
// what /auth/register checks before it will create an account. Scoped to the guest
// session so a verification cannot be replayed from a different device.
func EmailVerifiedKey(guestSessionID, email string) string {
	return fmt.Sprintf(constants.RedisEmailVerifiedKeyPattern, guestSessionID, email)
}

func CaptchaRequiredKey(ip string) string {
	return fmt.Sprintf(constants.RedisCaptchaRequiredKeyPattern, ip)
}
