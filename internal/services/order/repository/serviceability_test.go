package repository

import "testing"

func TestDeliveryFeeForDistance(t *testing.T) {
	tests := []struct {
		distance float64
		want     int64
	}{
		{distance: 0, want: 3000},
		{distance: 3, want: 3000},
		{distance: 3.1, want: 3500},
		{distance: 8.2, want: 6000},
		{distance: 30, want: 8000},
	}
	for _, tt := range tests {
		if got := deliveryFeeForDistance(tt.distance); got != tt.want {
			t.Fatalf("deliveryFeeForDistance(%v) = %d, want %d", tt.distance, got, tt.want)
		}
	}
}
