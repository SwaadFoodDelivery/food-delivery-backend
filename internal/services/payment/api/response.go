package api

import "food-delivery-backend/internal/services/payment/models"

type paymentResponse struct {
	PaymentID         string `json:"payment_id"`
	OrderID           string `json:"order_id"`
	Provider          string `json:"provider"`
	Status            string `json:"status"`
	ProviderPaymentID string `json:"provider_payment_id,omitempty"`
	FailureCode       string `json:"failure_code,omitempty"`
	FailureMessage    string `json:"failure_message,omitempty"`
	Amount            int64  `json:"amount_minor"`
	Currency          string `json:"currency"`
	Replay            bool   `json:"idempotency_replay,omitempty"`
}

func mapPayment(payment models.Payment, replay bool) paymentResponse {
	return paymentResponse{PaymentID: payment.PaymentID.String(), OrderID: payment.OrderID.String(), Provider: payment.Provider, Status: payment.Status, ProviderPaymentID: payment.ProviderPaymentID, FailureCode: payment.FailureCode, FailureMessage: payment.FailureMessage, Amount: payment.Amount, Currency: payment.Currency, Replay: replay}
}
