package api

type placeResponse struct {
	OrderID           string `json:"order_id"`
	Status            string `json:"status"`
	TotalAmount       int64  `json:"total_amount_minor"`
	Currency          string `json:"currency"`
	EstimatedDelivery string `json:"estimated_delivery_at"`
	Replay            bool   `json:"idempotency_replay,omitempty"`
}
