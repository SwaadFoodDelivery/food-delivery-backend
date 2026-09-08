package api

type quoteRequest struct {
	CartToken string `json:"cart_token"`
	AddressID string `json:"address_id"`
}

type placeRequest struct {
	CartToken     string `json:"cart_token"`
	AddressID     string `json:"address_id"`
	PaymentMethod string `json:"payment_method"`
	Instructions  string `json:"instructions"`
}
