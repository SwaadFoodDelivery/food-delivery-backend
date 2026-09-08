package utils

import "testing"

func TestHMACRoundTripAndTamperResistance(t *testing.T) {
	signature := SignHMAC("cart-id", "test-secret")
	if signature == "" || !VerifyHMAC("cart-id", signature, "test-secret") {
		t.Fatal("expected signature to verify")
	}
	if VerifyHMAC("other-cart", signature, "test-secret") || VerifyHMAC("cart-id", signature, "other-secret") || VerifyHMAC("cart-id", "not-hex", "test-secret") {
		t.Fatal("expected tampered signatures to fail")
	}
}
