package repository

import "testing"

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		valid  bool
	}{
		{name: "empty filter", status: "", valid: true},
		{name: "active order", status: "preparing", valid: true},
		{name: "delivery state", status: "out_for_delivery", valid: true},
		{name: "cancelled order", status: "cancelled", valid: true},
		{name: "unknown state", status: "deliveredish", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatus(tt.status)
			if (err == nil) != tt.valid {
				t.Fatalf("ValidateStatus(%q) error = %v, valid = %v", tt.status, err, tt.valid)
			}
		})
	}
}
