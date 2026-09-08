package repository

import (
	"testing"

	"food-delivery-backend/internal/services/delivery/models"
)

func TestValidDriverTransitionFollowsMockProgression(t *testing.T) {
	cases := []struct {
		current string
		next    string
	}{
		{models.StatusAssigned, models.StatusEnRouteToRestaurant},
		{models.StatusEnRouteToRestaurant, models.StatusArrivedAtRestaurant},
		{models.StatusArrivedAtRestaurant, models.StatusPickedUp},
		{models.StatusPickedUp, models.StatusOutForDelivery},
		{models.StatusOutForDelivery, models.StatusDelivered},
	}
	for _, tc := range cases {
		if !validDriverTransition(tc.current, tc.next) {
			t.Errorf("expected %s -> %s to be valid", tc.current, tc.next)
		}
	}

	if validDriverTransition(models.StatusAssigned, models.StatusDelivered) {
		t.Fatal("driver must not skip mock delivery states")
	}
}
