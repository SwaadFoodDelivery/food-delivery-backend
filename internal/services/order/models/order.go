package models

import (
	"time"

	cartmodels "food-delivery-backend/internal/services/cart/models"
	"github.com/google/uuid"
)

const (
	TaxRatePercent       int64 = 6
	DeliveryFeeMinor     int64 = 3000
	EstimatedDeliveryMin       = 35
)

type QuoteInput struct {
	UserID    uuid.UUID
	CartToken string
	AddressID uuid.UUID
}

type Quote struct {
	Subtotal            int64          `json:"subtotal_minor"`
	Taxes               int64          `json:"taxes_minor"`
	DeliveryFee         int64          `json:"delivery_fee_minor"`
	Discount            int64          `json:"discount_minor"`
	TotalAmount         int64          `json:"total_amount_minor"`
	Currency            string         `json:"currency"`
	EstimatedDeliveryAt time.Time      `json:"estimated_delivery_at"`
	Serviceability      Serviceability `json:"serviceability"`
}

type Serviceability struct {
	Serviceable          bool    `json:"serviceable"`
	ReasonCode           string  `json:"reason_code"`
	Reason               string  `json:"reason"`
	DistanceKM           float64 `json:"distance_km"`
	ServiceRadiusKM      float64 `json:"service_radius_km"`
	DeliveryFee          int64   `json:"delivery_fee_minor"`
	EstimatedDeliveryMin int     `json:"estimated_delivery_min"`
	Currency             string  `json:"currency"`
}

type PlaceInput struct {
	UserID         uuid.UUID
	CartToken      string
	AddressID      uuid.UUID
	PaymentMethod  string
	Instructions   string
	IdempotencyKey string
}

type Order struct {
	OrderID           uuid.UUID         `json:"order_id"`
	Status            string            `json:"status"`
	RestaurantID      uuid.UUID         `json:"restaurant_id"`
	RestaurantName    string            `json:"restaurant_name"`
	Subtotal          int64             `json:"subtotal_minor"`
	Taxes             int64             `json:"taxes_minor"`
	DeliveryFee       int64             `json:"delivery_fee_minor"`
	Discount          int64             `json:"discount_minor"`
	TotalAmount       int64             `json:"total_amount_minor"`
	Currency          string            `json:"currency"`
	PaymentMethod     string            `json:"payment_method"`
	AddressID         uuid.UUID         `json:"address_id"`
	Instructions      string            `json:"instructions,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	EstimatedDelivery time.Time         `json:"estimated_delivery_at"`
	Items             []cartmodels.Item `json:"items"`
}

type HistoryItem struct {
	OrderID        uuid.UUID `json:"order_id"`
	Status         string    `json:"status"`
	RestaurantName string    `json:"restaurant_name"`
	TotalAmount    int64     `json:"total_amount_minor"`
	Currency       string    `json:"currency"`
	PaymentMethod  string    `json:"payment_method"`
	DeliveryStatus string    `json:"delivery_status,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type StatusEvent struct {
	FromStatus string    `json:"from_status,omitempty"`
	ToStatus   string    `json:"to_status"`
	ChangedAt  time.Time `json:"changed_at"`
}

type History struct {
	OrderID        uuid.UUID     `json:"order_id"`
	Status         string        `json:"status"`
	OrderStatus    []StatusEvent `json:"order_status"`
	DeliveryStatus []StatusEvent `json:"delivery_status"`
}
