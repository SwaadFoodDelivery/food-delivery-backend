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

// Earnings is a driver's lifetime payout total in this demo economy: the sum
// of the delivery_fee on every order whose delivery this driver completed.
// There is no separate commission/payout-rate concept in this schema -- the
// customer-facing delivery fee is the payout, by design.
type Earnings struct {
	TotalEarnings  int64  `json:"total_earnings_minor"`
	DeliveredCount int    `json:"delivered_count"`
	Currency       string `json:"currency"`
	DemoLabel      string `json:"demo_label"`
}

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
