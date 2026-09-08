package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"food-delivery-backend/internal/services/cart/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type PostgresRepository struct{ db *sqlx.DB }

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository { return &PostgresRepository{db: db} }

type itemRow struct {
	ItemID       uuid.UUID `db:"item_id"`
	RestaurantID uuid.UUID `db:"restaurant_id"`
	Name         string    `db:"name"`
	Price        string    `db:"price"`
	Restaurant   string    `db:"restaurant_name"`
	Available    bool      `db:"is_available"`
}

type cartRow struct {
	CartID       uuid.UUID      `db:"cart_id"`
	RestaurantID *uuid.UUID     `db:"restaurant_id"`
	Restaurant   sql.NullString `db:"restaurant_name"`
}

func (r *PostgresRepository) AddItem(ctx context.Context, in AddItemInput) (AddItemOutput, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return AddItemOutput{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var item itemRow
	if err := tx.GetContext(ctx, &item, `
		SELECT i.item_id, i.restaurant_id, i.name, i.price::text, r.name AS restaurant_name, i.is_available
		FROM menu_items i
		JOIN restaurants r ON r.restaurant_id = i.restaurant_id AND r.status = 'active'
		WHERE i.item_id = $1 AND i.is_deleted = FALSE
		FOR UPDATE OF i`, in.ItemID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AddItemOutput{}, ErrItemNotFound
		}
		return AddItemOutput{}, err
	}
	if item.RestaurantID != in.RestaurantID {
		return AddItemOutput{}, ErrRestaurantMismatch
	}
	if !item.Available {
		return AddItemOutput{}, ErrItemUnavailable
	}

	var cartID uuid.UUID
	if in.CartID != nil {
		var cart cartRow
		if err := tx.GetContext(ctx, &cart, `
			SELECT c.cart_id, c.restaurant_id, r.name AS restaurant_name
			FROM carts c
			LEFT JOIN restaurants r ON r.restaurant_id = c.restaurant_id
			WHERE c.cart_id = $1 AND c.user_id = $2 AND c.status = 'active'
			  AND (c.expires_at IS NULL OR c.expires_at > NOW())
			FOR UPDATE`, *in.CartID, in.UserID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return AddItemOutput{}, ErrCartNotFound
			}
			return AddItemOutput{}, err
		}
		if cart.RestaurantID != nil && *cart.RestaurantID != in.RestaurantID {
			return AddItemOutput{}, ErrRestaurantMismatch
		}
		cartID = cart.CartID
		if cart.RestaurantID == nil {
			if _, err := tx.ExecContext(ctx, `UPDATE carts SET restaurant_id = $1, updated_at = NOW() WHERE cart_id = $2`, in.RestaurantID, cartID); err != nil {
				return AddItemOutput{}, err
			}
		}
	} else {
		deviceID := strings.TrimSpace(in.DeviceID)
		if deviceID == "" {
			deviceID = "authenticated"
		}
		if err := tx.GetContext(ctx, &cartID, `
			INSERT INTO carts (user_id, device_id, restaurant_id, status, expires_at)
			VALUES ($1, $2, $3, 'active', NOW() + INTERVAL '24 hours')
			RETURNING cart_id`, in.UserID, deviceID, in.RestaurantID); err != nil {
			return AddItemOutput{}, err
		}
	}

	customisations, err := json.Marshal(in.Customisations)
	if err != nil {
		return AddItemOutput{}, err
	}
	var cartItemID uuid.UUID
	if err := tx.GetContext(ctx, &cartItemID, `
		INSERT INTO cart_items (cart_id, item_id, quantity, customisations)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING cart_item_id`, cartID, in.ItemID, in.Quantity, string(customisations)); err != nil {
		return AddItemOutput{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE carts SET updated_at = NOW() WHERE cart_id = $1`, cartID); err != nil {
		return AddItemOutput{}, err
	}
	if err := tx.Commit(); err != nil {
		return AddItemOutput{}, err
	}
	return AddItemOutput{CartID: cartID, CartItemID: cartItemID}, nil
}

type cartItemRow struct {
	CartItemID     uuid.UUID       `db:"cart_item_id"`
	ItemID         uuid.UUID       `db:"item_id"`
	Name           string          `db:"name"`
	Price          string          `db:"price"`
	Quantity       int             `db:"quantity"`
	Customisations json.RawMessage `db:"customisations"`
}

func (r *PostgresRepository) Get(ctx context.Context, userID, cartID uuid.UUID) (models.Cart, error) {
	var meta cartRow
	if err := r.db.GetContext(ctx, &meta, `
		SELECT c.cart_id, c.restaurant_id, r.name AS restaurant_name
		FROM carts c
		LEFT JOIN restaurants r ON r.restaurant_id = c.restaurant_id
		WHERE c.cart_id = $1 AND c.user_id = $2 AND c.status = 'active'
		  AND (c.expires_at IS NULL OR c.expires_at > NOW())`, cartID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Cart{}, ErrCartNotFound
		}
		return models.Cart{}, err
	}
	var rows []cartItemRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT ci.cart_item_id, ci.item_id, i.name, i.price::text, ci.quantity, ci.customisations
		FROM cart_items ci
		JOIN menu_items i ON i.item_id = ci.item_id AND i.is_deleted = FALSE
		WHERE ci.cart_id = $1
		ORDER BY ci.added_at, ci.cart_item_id`, cartID); err != nil {
		return models.Cart{}, err
	}
	out := models.Cart{CartID: meta.CartID, RestaurantID: meta.RestaurantID, RestaurantName: nullString(meta.Restaurant), Items: make([]models.Item, 0, len(rows)), Currency: "INR"}
	for _, row := range rows {
		unit, err := decimalToMinor(row.Price)
		if err != nil {
			return models.Cart{}, fmt.Errorf("invalid menu price for item %s: %w", row.ItemID, err)
		}
		customisations := []string{}
		if len(row.Customisations) > 0 && string(row.Customisations) != "null" {
			if err := json.Unmarshal(row.Customisations, &customisations); err != nil {
				return models.Cart{}, fmt.Errorf("invalid customisations for cart item %s: %w", row.CartItemID, err)
			}
		}
		lineTotal, ok := safeMultiply(unit, int64(row.Quantity))
		if !ok {
			return models.Cart{}, fmt.Errorf("cart item total overflow")
		}
		out.Items = append(out.Items, models.Item{CartItemID: row.CartItemID, ItemID: row.ItemID, Name: row.Name, Quantity: row.Quantity, UnitPrice: unit, LineTotal: lineTotal, Customisations: customisations})
		if lineTotal > int64(^uint64(0)>>1)-out.Subtotal {
			return models.Cart{}, fmt.Errorf("cart subtotal overflow")
		}
		out.Subtotal += lineTotal
	}
	return out, nil
}

func (r *PostgresRepository) DeleteItem(ctx context.Context, userID, cartID, cartItemID uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var locked uuid.UUID
	if err := tx.GetContext(ctx, &locked, `SELECT cart_id FROM carts WHERE cart_id = $1 AND user_id = $2 AND status = 'active' AND (expires_at IS NULL OR expires_at > NOW()) FOR UPDATE`, cartID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCartNotFound
		}
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM cart_items WHERE cart_item_id = $1 AND cart_id = $2`, cartItemID, cartID)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return ErrCartItemNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE carts SET restaurant_id = CASE WHEN EXISTS (SELECT 1 FROM cart_items WHERE cart_id = $1) THEN restaurant_id ELSE NULL END, updated_at = NOW() WHERE cart_id = $1`, cartID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PostgresRepository) Clear(ctx context.Context, userID, cartID uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var locked uuid.UUID
	if err := tx.GetContext(ctx, &locked, `SELECT cart_id FROM carts WHERE cart_id = $1 AND user_id = $2 AND status = 'active' AND (expires_at IS NULL OR expires_at > NOW()) FOR UPDATE`, cartID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCartNotFound
		}
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cart_items WHERE cart_id = $1`, cartID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE carts SET restaurant_id = NULL, updated_at = NOW() WHERE cart_id = $1`, cartID); err != nil {
		return err
	}
	return tx.Commit()
}

func decimalToMinor(raw string) (int64, error) {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) > 2 || len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("invalid decimal %q", raw)
	}
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
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, fmt.Errorf("invalid whole amount")
	}
	minor, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid fractional amount")
	}
	return whole*100 + minor, nil
}

func safeMultiply(a, b int64) (int64, bool) {
	if a < 0 || b < 0 || (b != 0 && a > (int64(^uint64(0)>>1))/b) {
		return 0, false
	}
	return a * b, true
}

func nullString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}
