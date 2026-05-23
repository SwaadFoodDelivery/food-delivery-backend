package redis

import "fmt"

const (
	SESSION_KEY        = "session:%s"
	CART_KEY           = "cart:%s"
	OTP_KEY            = "otp:%s"
	OTP_RATE_LIMIT_KEY = "otp:rate:%s"
	CAPTCHA_KEY        = "captcha:required:%s"
)

func SessionKey(sessionID string) string {
	return fmt.Sprintf(SESSION_KEY, sessionID)
}

func CartKey(userID string) string {
	return fmt.Sprintf(CART_KEY, userID)
}

func OTPKey(phone string) string {
	return fmt.Sprintf(OTP_KEY, phone)
}

func OTPRateKey(phone string) string {
	return fmt.Sprintf(OTP_RATE_LIMIT_KEY, phone)
}

func CaptchaRequiredKey(ip string) string {
	return fmt.Sprintf(CAPTCHA_KEY, ip)
}
