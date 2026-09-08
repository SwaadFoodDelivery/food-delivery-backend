package repository

import "testing"

func TestValidOrderTransition(t *testing.T) {
	tests := []struct {
		current string
		next    string
		valid   bool
	}{
		{current: "order_created", next: "accepted", valid: true},
		{current: "confirmed", next: "rejected", valid: true},
		{current: "accepted", next: "preparing", valid: true},
		{current: "preparing", next: "ready_for_pickup", valid: true},
		{current: "delivered", next: "preparing", valid: false},
		{current: "order_created", next: "delivered", valid: false},
	}
	for _, test := range tests {
		if got := validOrderTransition(test.current, test.next); got != test.valid {
			t.Errorf("validOrderTransition(%q, %q) = %v, want %v", test.current, test.next, got, test.valid)
		}
	}
}
