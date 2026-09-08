package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusPending = "pending"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

type Payment struct {
	PaymentID         uuid.UUID `json:"payment_id"`
	OrderID           uuid.UUID `json:"order_id"`
	UserID            uuid.UUID `json:"-"`
	Provider          string    `json:"provider"`
	Status            string    `json:"status"`
	ProviderPaymentID string    `json:"provider_payment_id,omitempty"`
	FailureCode       string    `json:"failure_code,omitempty"`
	FailureMessage    string    `json:"failure_message,omitempty"`
	Amount            int64     `json:"amount_minor"`
	Currency          string    `json:"currency"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Order struct {
	OrderID       uuid.UUID
	CreatedAt     time.Time
	UserID        uuid.UUID
	TotalAmount   int64
	PaymentMethod string
	Status        string
}
