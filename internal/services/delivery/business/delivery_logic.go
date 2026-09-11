package business

import "food-delivery-backend/internal/services/delivery/models"

// Statuses documents the backend-owned mock progression used by the demo UI.
var Statuses = []string{
	models.StatusAssigned,
	models.StatusEnRouteToRestaurant,
	models.StatusArrivedAtRestaurant,
	models.StatusPickedUp,
	models.StatusOutForDelivery,
	models.StatusDelivered,
}
