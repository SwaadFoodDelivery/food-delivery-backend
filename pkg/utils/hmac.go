package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func SignHMAC(msg, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyHMAC(msg, sig, key string) bool {
	provided, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(msg))
	return hmac.Equal(mac.Sum(nil), provided)
}
