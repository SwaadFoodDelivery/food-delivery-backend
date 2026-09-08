package api

type addItemResponse struct {
	CartToken  string `json:"cart_token"`
	CartItemID string `json:"cart_item_id"`
	Message    string `json:"message"`
}
