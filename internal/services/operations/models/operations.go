package models

import (
	"time"

	"github.com/google/uuid"
)

type Overview struct {
	Summary Summary  `json:"summary"`
	Orders  []Order  `json:"orders"`
	Drivers []Driver `json:"drivers"`
}

type Summary struct {
	TotalOrders      int `json:"total_orders"`
	ActiveOrders     int `json:"active_orders"`
	DeliveredOrders  int `json:"delivered_orders"`
	AvailableDrivers int `json:"available_drivers"`
	ActiveDrivers    int `json:"active_drivers"`
}

type Order struct {
	OrderID        uuid.UUID `json:"order_id"`
	Status         string    `json:"status"`
	TotalAmount    int64     `json:"total_amount_minor"`
	Currency       string    `json:"currency"`
	PaymentMethod  string    `json:"payment_method"`
	CustomerName   string    `json:"customer_name"`
	RestaurantName string    `json:"restaurant_name"`
	DeliveryStatus string    `json:"delivery_status,omitempty"`
	PartnerName    string    `json:"partner_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Driver struct {
	DriverID       uuid.UUID  `json:"driver_id"`
	Name           string     `json:"name"`
	IsAvailable    bool       `json:"is_available"`
	CurrentCity    string     `json:"current_city,omitempty"`
	ActiveOrderID  *uuid.UUID `json:"active_order_id,omitempty"`
	DeliveryStatus string     `json:"delivery_status,omitempty"`
}
