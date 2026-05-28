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

func OTPKey(phone string) string {
	return fmt.Sprintf(constants.RedisOTPKeyPattern, phone)
}

func OTPRateKey(phone string) string {
	return fmt.Sprintf(constants.RedisOTPRateLimitKeyPattern, phone)
}

func CaptchaRequiredKey(ip string) string {
	return fmt.Sprintf(constants.RedisCaptchaRequiredKeyPattern, ip)
}
