package repository

import "testing"

func TestCursorRoundTrip(t *testing.T) {
	for _, offset := range []int{0, 20, 1000} {
		got, err := DecodeCursor(EncodeCursor(offset))
		if err != nil {
			t.Fatalf("offset %d: %v", offset, err)
		}
		if got != offset {
			t.Fatalf("got %d, want %d", got, offset)
		}
	}
}

func TestDecodeCursorRejectsInvalidValues(t *testing.T) {
	for _, raw := range []string{"not-base64", "LTE", "-1"} {
		if _, err := DecodeCursor(raw); err == nil {
			t.Fatalf("expected %q to fail", raw)
		}
	}
}
