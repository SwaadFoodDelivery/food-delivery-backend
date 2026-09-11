package api

import "github.com/google/uuid"

type addItemRequest struct {
	CartToken      string   `json:"cart_token"`
	RestaurantID   string   `json:"restaurant_id"`
	ItemID         string   `json:"item_id"`
	Quantity       int      `json:"quantity"`
	Customisations []string `json:"customisations"`
}

func (r addItemRequest) IDs() (uuid.UUID, uuid.UUID, error) {
	restaurantID, err := uuid.Parse(r.RestaurantID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	itemID, err := uuid.Parse(r.ItemID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return restaurantID, itemID, nil
}
