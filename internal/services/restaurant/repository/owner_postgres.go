package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"food-delivery-backend/internal/services/restaurant/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

func (r *PostgresRepository) CreateItem(ctx context.Context, actorID, restaurantID, categoryID uuid.UUID, item models.Item) (models.Item, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return models.Item{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := r.verifyOwnerCategory(ctx, tx, actorID, restaurantID, categoryID); err != nil {
		return models.Item{}, err
	}
	var row itemMutationRow
	err = tx.GetContext(ctx, &row, `
		INSERT INTO menu_items (restaurant_id, category_id, name, description, price, image_s3_key, is_veg, is_available, preparation_time_min, tags)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5::decimal, NULLIF($6, ''), $7, $8, $9, $10)
		RETURNING item_id, name, description, price::text, image_s3_key, is_veg, is_available, preparation_time_min, tags`,
		restaurantID, categoryID, item.Name, item.Description, ownerMinorDecimal(item.Price), item.ImageURL, item.IsVeg, item.IsAvailable, item.PreparationTimeMin, pq.StringArray(item.Tags))
	if err != nil {
		return models.Item{}, err
	}
	out := mapMutationItem(row)
	if err := insertMenuAudit(ctx, tx, actorID, "menu_item_created", out.ItemID, nil, out); err != nil {
		return models.Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.Item{}, err
	}
	return out, nil
}

func (r *PostgresRepository) UpdateItem(ctx context.Context, actorID, restaurantID, itemID uuid.UUID, item models.Item) (models.Item, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return models.Item{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var before itemMutationRow
	if err := tx.GetContext(ctx, &before, `
		SELECT i.item_id, i.name, i.description, i.price::text, i.image_s3_key, i.is_veg, i.is_available, i.preparation_time_min, i.tags
		FROM menu_items i JOIN restaurants r ON r.restaurant_id = i.restaurant_id
		WHERE i.item_id = $1 AND r.restaurant_id = $2 AND r.owner_id = $3 AND r.status = 'active' AND i.is_deleted = FALSE FOR UPDATE`, itemID, restaurantID, actorID); err != nil {
		return models.Item{}, ownerLookupError(err)
	}
	var after itemMutationRow
	if err := tx.GetContext(ctx, &after, `
		UPDATE menu_items
		SET name = $1, description = NULLIF($2, ''), price = $3::decimal, image_s3_key = NULLIF($4, ''),
		    is_veg = $5, is_available = $6, preparation_time_min = $7, tags = $8, updated_at = NOW()
		WHERE item_id = $9
		RETURNING item_id, name, description, price::text, image_s3_key, is_veg, is_available, preparation_time_min, tags`,
		item.Name, item.Description, ownerMinorDecimal(item.Price), item.ImageURL, item.IsVeg, item.IsAvailable, item.PreparationTimeMin, pq.StringArray(item.Tags), itemID); err != nil {
		return models.Item{}, err
	}
	out := mapMutationItem(after)
	if err := insertMenuAudit(ctx, tx, actorID, "menu_item_updated", out.ItemID, mapMutationItem(before), out); err != nil {
		return models.Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.Item{}, err
	}
	return out, nil
}

func (r *PostgresRepository) DeleteItem(ctx context.Context, actorID, restaurantID, itemID uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var before itemMutationRow
	if err := tx.GetContext(ctx, &before, `
		SELECT i.item_id, i.name, i.description, i.price::text, i.image_s3_key, i.is_veg, i.is_available, i.preparation_time_min, i.tags
		FROM menu_items i JOIN restaurants r ON r.restaurant_id = i.restaurant_id
		WHERE i.item_id = $1 AND r.restaurant_id = $2 AND r.owner_id = $3 AND r.status = 'active' AND i.is_deleted = FALSE FOR UPDATE`, itemID, restaurantID, actorID); err != nil {
		return ownerLookupError(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE menu_items SET is_deleted = TRUE, is_available = FALSE, updated_at = NOW() WHERE item_id = $1`, itemID); err != nil {
		return err
	}
	if err := insertMenuAudit(ctx, tx, actorID, "menu_item_deleted", itemID, mapMutationItem(before), nil); err != nil {
		return err
	}
	return tx.Commit()
}

type restaurantOrderRow struct {
	OrderID       uuid.UUID `db:"order_id"`
	Status        string    `db:"status"`
	TotalAmount   string    `db:"total_amount"`
	PaymentMethod string    `db:"payment_method"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

func (r *PostgresRepository) GetOwnedRestaurant(ctx context.Context, actorID uuid.UUID) (models.Restaurant, error) {
	var row restaurantRow
	if err := r.db.GetContext(ctx, &row, `
		SELECT r.restaurant_id, r.name, r.description, r.cuisine_types,
		       r.address_line1, r.area, r.city, r.state, r.pincode,
		       r.latitude, r.longitude, 0 AS distance_km,
		       r.service_radius_km, r.delivery_time_min, r.rating, r.total_ratings,
		       r.is_open, r.logo_s3_key AS restaurant_profile_image
		FROM restaurants r
		WHERE r.owner_id = $1 AND r.status = 'active'
		ORDER BY r.created_at ASC, r.restaurant_id ASC
		LIMIT 1`, actorID); err != nil {
		return models.Restaurant{}, err
	}
	return mapRestaurant(row), nil
}

func (r *PostgresRepository) ListOrders(ctx context.Context, actorID, restaurantID uuid.UUID) ([]models.RestaurantOrder, error) {
	var rows []restaurantOrderRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT o.order_id, o.status::text, o.total_amount::text, o.payment_method, o.created_at, o.updated_at
		FROM orders o JOIN restaurants r ON r.restaurant_id = o.restaurant_id
		WHERE o.restaurant_id = $1 AND r.owner_id = $2
		ORDER BY o.created_at DESC LIMIT 50`, restaurantID, actorID); err != nil {
		return nil, err
	}
	out := make([]models.RestaurantOrder, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapRestaurantOrder(row))
	}
	return out, nil
}

func (r *PostgresRepository) UpdateOrderStatus(ctx context.Context, actorID, restaurantID, orderID uuid.UUID, next string) (models.RestaurantOrder, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return models.RestaurantOrder{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var before restaurantOrderRow
	if err := tx.GetContext(ctx, &before, `
		SELECT o.order_id, o.status::text, o.total_amount::text, o.payment_method, o.created_at, o.updated_at
		FROM orders o JOIN restaurants r ON r.restaurant_id = o.restaurant_id
		WHERE o.order_id = $1 AND o.restaurant_id = $2 AND r.owner_id = $3
		ORDER BY o.created_at DESC LIMIT 1 FOR UPDATE`, orderID, restaurantID, actorID); err != nil {
		return models.RestaurantOrder{}, ownerOrderLookupError(err)
	}
	if !validOrderTransition(before.Status, next) {
		return models.RestaurantOrder{}, fmt.Errorf("order cannot move from %s to %s", before.Status, next)
	}
	var after restaurantOrderRow
	if err := tx.GetContext(ctx, &after, `
		UPDATE orders SET status = $1::order_status, updated_by = $2, updated_at = NOW()
		WHERE order_id = $3 AND restaurant_id = $4
		RETURNING order_id, status::text, total_amount::text, payment_method, created_at, updated_at`, next, actorID, orderID, restaurantID); err != nil {
		return models.RestaurantOrder{}, err
	}
	beforeJSON, err := json.Marshal(map[string]any{"status": before.Status})
	if err != nil {
		return models.RestaurantOrder{}, err
	}
	afterJSON, err := json.Marshal(map[string]any{"status": after.Status})
	if err != nil {
		return models.RestaurantOrder{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (actor_id, actor_role, action, entity_type, entity_id, before, after)
		VALUES ($1, 'restaurant_owner', 'order_status_updated', 'order', $2, $3::jsonb, $4::jsonb)`, actorID, orderID.String(), beforeJSON, afterJSON); err != nil {
		return models.RestaurantOrder{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.RestaurantOrder{}, err
	}
	return mapRestaurantOrder(after), nil
}

func mapRestaurantOrder(row restaurantOrderRow) models.RestaurantOrder {
	return models.RestaurantOrder{OrderID: row.OrderID, Status: row.Status, TotalAmount: ownerMustMinor(row.TotalAmount), Currency: "INR", PaymentMethod: row.PaymentMethod, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func validOrderTransition(current, next string) bool {
	switch current {
	case "order_created", "confirmed":
		return next == "accepted" || next == "rejected"
	case "accepted":
		return next == "preparing" || next == "rejected"
	case "preparing":
		return next == "ready_for_pickup"
	default:
		return false
	}
}

func ownerOrderLookupError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("order not found")
	}
	return err
}

type itemMutationRow struct {
	ItemID             uuid.UUID      `db:"item_id"`
	Name               string         `db:"name"`
	Description        sql.NullString `db:"description"`
	Price              string         `db:"price"`
	ImageURL           sql.NullString `db:"image_s3_key"`
	IsVeg              bool           `db:"is_veg"`
	IsAvailable        bool           `db:"is_available"`
	PreparationTimeMin int            `db:"preparation_time_min"`
	Tags               pq.StringArray `db:"tags"`
}

func (r *PostgresRepository) verifyOwnerCategory(ctx context.Context, tx *sqlx.Tx, actorID, restaurantID, categoryID uuid.UUID) error {
	var ok bool
	if err := tx.GetContext(ctx, &ok, `
		SELECT EXISTS (SELECT 1 FROM menu_categories c JOIN menus m ON m.menu_id = c.menu_id
		JOIN restaurants r ON r.restaurant_id = m.restaurant_id
		WHERE c.category_id = $1 AND m.restaurant_id = $2 AND r.owner_id = $3 AND r.status = 'active')`, categoryID, restaurantID, actorID); err != nil {
		return err
	}
	if !ok {
		return sql.ErrNoRows
	}
	return nil
}

func ownerLookupError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("menu item not found")
	}
	return err
}

func mapMutationItem(row itemMutationRow) models.Item {
	return models.Item{ItemID: row.ItemID, Name: row.Name, Description: nullString(row.Description), Price: ownerMustMinor(row.Price), Currency: "INR", IsVeg: row.IsVeg, IsAvailable: row.IsAvailable, PreparationTimeMin: row.PreparationTimeMin, Tags: row.Tags, ImageURL: nullString(row.ImageURL)}
}

func ownerMinorDecimal(value int64) string { return fmt.Sprintf("%d.%02d", value/100, value%100) }

func ownerMustMinor(raw string) int64 {
	value, _ := decimalToMinor(raw)
	return value
}

func insertMenuAudit(ctx context.Context, tx *sqlx.Tx, actorID uuid.UUID, action string, entityID uuid.UUID, before, after any) error {
	beforeJSON, err := auditJSON(before)
	if err != nil {
		return err
	}
	afterJSON, err := auditJSON(after)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs (actor_id, actor_role, action, entity_type, entity_id, before, after)
		VALUES ($1, 'restaurant_owner', $2, 'menu_item', $3, $4::jsonb, $5::jsonb)`, actorID, action, entityID.String(), beforeJSON, afterJSON)
	return err
}

func auditJSON(value any) ([]byte, error) {
	if value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(value)
}
