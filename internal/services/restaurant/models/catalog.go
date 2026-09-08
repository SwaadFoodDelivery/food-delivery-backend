package models

import "github.com/google/uuid"

type Restaurant struct {
	RestaurantID    uuid.UUID `json:"restaurant_id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	CuisineTypes    []string  `json:"cuisine_types"`
	AddressLine1    string    `json:"address_line1,omitempty"`
	Area            string    `json:"area,omitempty"`
	City            string    `json:"city,omitempty"`
	State           string    `json:"state,omitempty"`
	Pincode         string    `json:"pincode,omitempty"`
	Latitude        float64   `json:"latitude"`
	Longitude       float64   `json:"longitude"`
	DistanceKM      float64   `json:"distance_km"`
	ServiceRadiusKM float64   `json:"service_radius_km"`
	DeliveryTimeMin int       `json:"delivery_time_min"`
	Rating          float64   `json:"rating"`
	TotalRatings    int       `json:"total_ratings"`
	IsOpen          bool      `json:"is_open"`
	RestaurantImage string    `json:"restaurant_profile_image,omitempty"`
}

type Category struct {
	CategoryID uuid.UUID `json:"category_id"`
	Name       string    `json:"category"`
	SortOrder  int       `json:"sort_order"`
	Items      []Item    `json:"items"`
}

type Item struct {
	ItemID             uuid.UUID `json:"item_id"`
	Name               string    `json:"name"`
	Description        string    `json:"description,omitempty"`
	Price              int64     `json:"price_minor"`
	Currency           string    `json:"currency"`
	IsVeg              bool      `json:"is_veg"`
	IsAvailable        bool      `json:"is_available"`
	PreparationTimeMin int       `json:"preparation_time_min"`
	Tags               []string  `json:"tags"`
	ImageURL           string    `json:"image_url,omitempty"`
}

type RestaurantList struct {
	Restaurants []Restaurant `json:"restaurants"`
	Total       int          `json:"total"`
	NextCursor  string       `json:"next_cursor,omitempty"`
}

type Menu struct {
	RestaurantID uuid.UUID  `json:"restaurant_id"`
	Categories   []Category `json:"categories"`
}
