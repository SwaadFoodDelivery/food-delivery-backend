package models

import "github.com/google/uuid"

type Cart struct {
	CartToken      string     `json:"cart_token"`
	RestaurantID   *uuid.UUID `json:"restaurant_id,omitempty"`
	RestaurantName string     `json:"restaurant_name,omitempty"`
	Items          []Item     `json:"items"`
	Subtotal       int64      `json:"subtotal_minor"`
	Currency       string     `json:"currency"`
}

type Item struct {
	CartItemID     uuid.UUID `json:"cart_item_id"`
	ItemID         uuid.UUID `json:"item_id"`
	Name           string    `json:"name"`
	Quantity       int       `json:"quantity"`
	UnitPrice      int64     `json:"unit_price_minor"`
	LineTotal      int64     `json:"line_total_minor"`
	Customisations []string  `json:"customisations"`
}
