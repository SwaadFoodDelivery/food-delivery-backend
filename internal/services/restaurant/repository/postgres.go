package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"food-delivery-backend/internal/services/restaurant/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type PostgresRepository struct{ db *sqlx.DB }

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository { return &PostgresRepository{db: db} }

type restaurantRow struct {
	RestaurantID    uuid.UUID       `db:"restaurant_id"`
	Name            string          `db:"name"`
	Description     sql.NullString  `db:"description"`
	CuisineTypes    pq.StringArray  `db:"cuisine_types"`
	AddressLine1    sql.NullString  `db:"address_line1"`
	Area            sql.NullString  `db:"area"`
	City            sql.NullString  `db:"city"`
	State           sql.NullString  `db:"state"`
	Pincode         sql.NullString  `db:"pincode"`
	Latitude        sql.NullFloat64 `db:"latitude"`
	Longitude       sql.NullFloat64 `db:"longitude"`
	DistanceKM      sql.NullFloat64 `db:"distance_km"`
	ServiceRadiusKM float64         `db:"service_radius_km"`
	DeliveryTimeMin int             `db:"delivery_time_min"`
	Rating          float64         `db:"rating"`
	TotalRatings    int             `db:"total_ratings"`
	IsOpen          bool            `db:"is_open"`
	RestaurantImage sql.NullString  `db:"restaurant_profile_image"`
}

func (r *PostgresRepository) List(ctx context.Context, filter SearchFilter) (models.RestaurantList, error) {
	orderBy := "distance_km ASC, restaurant_id ASC"
	if filter.SortBy == "rating" {
		orderBy = "rating DESC, distance_km ASC, restaurant_id ASC"
	}
	args := []any{filter.Longitude, filter.Latitude, filter.RadiusKM * 1000}
	where := []string{
		"r.status = 'active'",
		"ST_DWithin(r.location::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)",
	}
	if filter.Cuisine != "" {
		args = append(args, filter.Cuisine)
		where = append(where, "EXISTS (SELECT 1 FROM unnest(r.cuisine_types) cuisine WHERE lower(cuisine) = lower($4))")
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db.GetContext(ctx, &total, fmt.Sprintf("SELECT count(*) FROM restaurants r WHERE %s", whereSQL), args...); err != nil {
		return models.RestaurantList{}, err
	}
	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	args = append(args, filter.Limit, filter.Offset)
	query := fmt.Sprintf(`
		SELECT r.restaurant_id, r.name, r.description, r.cuisine_types,
		       r.address_line1, r.area, r.city, r.state, r.pincode,
		       r.latitude, r.longitude,
		       ST_Distance(r.location::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) / 1000 AS distance_km,
		       r.service_radius_km, r.delivery_time_min, r.rating, r.total_ratings,
		       r.is_open, r.logo_s3_key AS restaurant_profile_image
		FROM restaurants r
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, whereSQL, orderBy, limitArg, offsetArg)
	var rows []restaurantRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return models.RestaurantList{}, err
	}
	out := models.RestaurantList{Restaurants: make([]models.Restaurant, 0, len(rows)), Total: total}
	for _, row := range rows {
		out.Restaurants = append(out.Restaurants, mapRestaurant(row))
	}
	if filter.Offset+len(rows) < total {
		out.NextCursor = EncodeCursor(filter.Offset + len(rows))
	}
	return out, nil
}

func (r *PostgresRepository) Get(ctx context.Context, restaurantID uuid.UUID, latitude, longitude *float64) (models.Restaurant, error) {
	distanceSQL := "NULL::double precision"
	args := []any{restaurantID}
	if latitude != nil && longitude != nil {
		distanceSQL = "ST_Distance(r.location::geography, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography) / 1000"
		args = append(args, *longitude, *latitude)
	}
	query := fmt.Sprintf(`
		SELECT r.restaurant_id, r.name, r.description, r.cuisine_types,
		       r.address_line1, r.area, r.city, r.state, r.pincode,
		       r.latitude, r.longitude, %s AS distance_km,
		       r.service_radius_km, r.delivery_time_min, r.rating, r.total_ratings,
		       r.is_open, r.logo_s3_key AS restaurant_profile_image
		FROM restaurants r
		WHERE r.restaurant_id = $1 AND r.status = 'active'`, distanceSQL)
	var row restaurantRow
	if err := r.db.GetContext(ctx, &row, query, args...); err != nil {
		return models.Restaurant{}, err
	}
	return mapRestaurant(row), nil
}

type menuRow struct {
	CategoryID         uuid.UUID      `db:"category_id"`
	CategoryName       string         `db:"category_name"`
	CategorySortOrder  int            `db:"category_sort_order"`
	ItemID             *uuid.UUID     `db:"item_id"`
	ItemName           sql.NullString `db:"item_name"`
	Description        sql.NullString `db:"description"`
	Price              string         `db:"price"`
	ImageS3Key         sql.NullString `db:"image_s3_key"`
	IsVeg              sql.NullBool   `db:"is_veg"`
	IsAvailable        sql.NullBool   `db:"is_available"`
	PreparationTimeMin sql.NullInt64  `db:"preparation_time_min"`
	Tags               pq.StringArray `db:"tags"`
	ItemSortOrder      sql.NullInt64  `db:"item_sort_order"`
}

func (r *PostgresRepository) Menu(ctx context.Context, restaurantID uuid.UUID) (models.Menu, error) {
	const query = `
		SELECT c.category_id, c.name AS category_name, c.sort_order AS category_sort_order,
		       i.item_id, i.name AS item_name, i.description, i.price, i.image_s3_key,
		       i.is_veg, i.is_available, i.preparation_time_min, i.tags, i.sort_order AS item_sort_order
		FROM menus m
		JOIN restaurants r ON r.restaurant_id = m.restaurant_id AND r.status = 'active'
		JOIN menu_categories c ON c.menu_id = m.menu_id
		LEFT JOIN menu_items i ON i.category_id = c.category_id AND i.is_deleted = FALSE
		WHERE m.restaurant_id = $1
		ORDER BY c.sort_order, c.name, i.sort_order, i.name`
	var rows []menuRow
	if err := r.db.SelectContext(ctx, &rows, query, restaurantID); err != nil {
		return models.Menu{}, err
	}
	out := models.Menu{RestaurantID: restaurantID, Categories: []models.Category{}}
	categoryIndex := map[uuid.UUID]int{}
	for _, row := range rows {
		idx, ok := categoryIndex[row.CategoryID]
		if !ok {
			idx = len(out.Categories)
			categoryIndex[row.CategoryID] = idx
			out.Categories = append(out.Categories, models.Category{CategoryID: row.CategoryID, Name: row.CategoryName, SortOrder: row.CategorySortOrder, Items: []models.Item{}})
		}
		if row.ItemID == nil {
			continue
		}
		priceMinor, err := decimalToMinor(row.Price)
		if err != nil {
			return models.Menu{}, fmt.Errorf("invalid menu price for item %s: %w", row.ItemID, err)
		}
		out.Categories[idx].Items = append(out.Categories[idx].Items, models.Item{
			ItemID: *row.ItemID, Name: nullString(row.ItemName), Description: nullString(row.Description),
			Price: priceMinor, Currency: "INR", IsVeg: row.IsVeg.Bool,
			IsAvailable: row.IsAvailable.Bool, PreparationTimeMin: int(row.PreparationTimeMin.Int64), Tags: row.Tags,
			ImageURL: nullString(row.ImageS3Key),
		})
	}
	return out, nil
}

func decimalToMinor(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("invalid decimal %q", raw)
	}
	whole := parts[0]
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		return 0, fmt.Errorf("more than two decimal places")
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	wholeValue, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || wholeValue < 0 {
		return 0, fmt.Errorf("invalid whole amount")
	}
	fractionValue, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid fractional amount")
	}
	return wholeValue*100 + fractionValue, nil
}

func mapRestaurant(row restaurantRow) models.Restaurant {
	return models.Restaurant{RestaurantID: row.RestaurantID, Name: row.Name, Description: nullString(row.Description), CuisineTypes: []string(row.CuisineTypes),
		AddressLine1: nullString(row.AddressLine1), Area: nullString(row.Area), City: nullString(row.City), State: nullString(row.State), Pincode: nullString(row.Pincode),
		Latitude: row.Latitude.Float64, Longitude: row.Longitude.Float64, DistanceKM: row.DistanceKM.Float64, ServiceRadiusKM: row.ServiceRadiusKM,
		DeliveryTimeMin: row.DeliveryTimeMin, Rating: row.Rating, TotalRatings: row.TotalRatings, IsOpen: row.IsOpen, RestaurantImage: nullString(row.RestaurantImage)}
}

func nullString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}
