package models

import (
	"time"

	"github.com/google/uuid"
)

type RestaurantOrder struct {
	OrderID       uuid.UUID `json:"order_id"`
	Status        string    `json:"status"`
	TotalAmount   int64     `json:"total_amount_minor"`
	Currency      string    `json:"currency"`
	PaymentMethod string    `json:"payment_method"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
