package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusAssigned            = "assigned"
	StatusEnRouteToRestaurant = "en_route_to_restaurant"
	StatusArrivedAtRestaurant = "arrived_at_restaurant"
	StatusPickedUp            = "picked_up"
	StatusOutForDelivery      = "out_for_delivery"
	StatusDelivered           = "delivered"
)

type Delivery struct {
	DeliveryID       uuid.UUID  `json:"delivery_id"`
	OrderID          uuid.UUID  `json:"order_id"`
	Provider         string     `json:"provider"`
	Status           string     `json:"status"`
	PartnerID        uuid.UUID  `json:"partner_id"`
	PartnerName      string     `json:"partner_name"`
	PartnerPhone     string     `json:"partner_phone"`
	AssignedAt       time.Time  `json:"assigned_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	NextTransitionAt *time.Time `json:"next_transition_at,omitempty"`
	DemoLabel        string     `json:"demo_label"`
}
