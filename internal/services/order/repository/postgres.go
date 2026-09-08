package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	cartmodels "food-delivery-backend/internal/services/cart/models"
	ordermodels "food-delivery-backend/internal/services/order/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type PostgresRepository struct{ db *sqlx.DB }

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) FindByIdempotency(ctx context.Context, userID uuid.UUID, key string) (ordermodels.Order, bool, error) {
	var row orderRow
	err := r.db.GetContext(ctx, &row, `
		SELECT o.order_id, o.created_at, o.status::text, o.restaurant_id, r.name AS restaurant_name,
		       o.subtotal::text, o.taxes::text, o.delivery_fee::text, o.discount::text,
		       o.total_amount::text, o.payment_method, o.address_id, o.instructions
		FROM orders o JOIN restaurants r ON r.restaurant_id = o.restaurant_id
		WHERE o.user_id = $1 AND o.idempotency_key = $2
		ORDER BY o.created_at DESC LIMIT 1`, userID, key)
	if errors.Is(err, sql.ErrNoRows) {
		return ordermodels.Order{}, false, nil
	}
	if err != nil {
		return ordermodels.Order{}, false, err
	}
	out, err := r.mapOrder(ctx, nil, row)
	return out, true, err
}

type addressRow struct {
	AddressID uuid.UUID       `db:"address_id"`
	Latitude  sql.NullFloat64 `db:"latitude"`
	Longitude sql.NullFloat64 `db:"longitude"`
}

type itemRow struct {
	ItemID       uuid.UUID `db:"item_id"`
	RestaurantID uuid.UUID `db:"restaurant_id"`
	Name         string    `db:"name"`
	Price        string    `db:"price"`
	Available    bool      `db:"is_available"`
}

type orderRow struct {
	OrderID       uuid.UUID      `db:"order_id"`
	CreatedAt     time.Time      `db:"created_at"`
	Status        string         `db:"status"`
	RestaurantID  uuid.UUID      `db:"restaurant_id"`
	Restaurant    string         `db:"restaurant_name"`
	Subtotal      string         `db:"subtotal"`
	Taxes         string         `db:"taxes"`
	DeliveryFee   string         `db:"delivery_fee"`
	Discount      string         `db:"discount"`
	TotalAmount   string         `db:"total_amount"`
	PaymentMethod string         `db:"payment_method"`
	AddressID     uuid.UUID      `db:"address_id"`
	Instructions  sql.NullString `db:"instructions"`
}

func (r *PostgresRepository) Quote(ctx context.Context, userID uuid.UUID, cart cartmodels.Cart, addressID uuid.UUID) (ordermodels.Quote, error) {
	if len(cart.Items) == 0 || cart.RestaurantID == nil {
		return ordermodels.Quote{}, ErrCartEmpty
	}
	if err := r.validateAddress(ctx, userID, addressID, *cart.RestaurantID); err != nil {
		return ordermodels.Quote{}, err
	}
	if err := r.validateItems(ctx, *cart.RestaurantID, cart.Items); err != nil {
		return ordermodels.Quote{}, err
	}
	return quoteFor(cart.Subtotal, time.Now().UTC()), nil
}

func (r *PostgresRepository) Place(ctx context.Context, in ordermodels.PlaceInput, cart cartmodels.Cart, quote ordermodels.Quote) (ordermodels.Order, bool, error) {
	if len(cart.Items) == 0 || cart.RestaurantID == nil {
		return ordermodels.Order{}, false, ErrCartEmpty
	}
	if strings.TrimSpace(in.PaymentMethod) == "" {
		return ordermodels.Order{}, false, ErrInvalidPayment
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return ordermodels.Order{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, in.UserID.String()+":"+in.IdempotencyKey); err != nil {
		return ordermodels.Order{}, false, err
	}
	var existing orderRow
	if err := tx.GetContext(ctx, &existing, `
		SELECT o.order_id, o.created_at, o.status::text, o.restaurant_id, r.name AS restaurant_name,
		       o.subtotal::text, o.taxes::text, o.delivery_fee::text, o.discount::text,
		       o.total_amount::text, o.payment_method, o.address_id, o.instructions
		FROM orders o JOIN restaurants r ON r.restaurant_id = o.restaurant_id
		WHERE o.user_id = $1 AND o.idempotency_key = $2
		ORDER BY o.created_at DESC LIMIT 1`, in.UserID, in.IdempotencyKey); err == nil {
		out, err := r.mapOrder(ctx, tx, existing)
		return out, true, err
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ordermodels.Order{}, false, err
	}
	lockedCart, err := r.lockActiveCart(ctx, tx, in.UserID, cart.CartID)
	if err != nil {
		return ordermodels.Order{}, false, err
	}
	if len(lockedCart.Items) == 0 || lockedCart.RestaurantID == nil {
		return ordermodels.Order{}, false, ErrCartEmpty
	}
	quote = quoteFor(lockedCart.Subtotal, time.Now().UTC())
	if err := r.validateAddressTx(ctx, tx, in.UserID, in.AddressID, *lockedCart.RestaurantID); err != nil {
		return ordermodels.Order{}, false, err
	}
	if err := r.validateItemsTx(ctx, tx, *lockedCart.RestaurantID, lockedCart.Items); err != nil {
		return ordermodels.Order{}, false, err
	}
	var row orderRow
	if err := tx.GetContext(ctx, &row, `
		INSERT INTO orders (user_id, restaurant_id, address_id, status, subtotal, taxes, delivery_fee, discount, total_amount, payment_method, idempotency_key, instructions)
		VALUES ($1, $2, $3, 'order_created', $4::decimal, $5::decimal, $6::decimal, $7::decimal, $8::decimal, $9, $10, $11)
		RETURNING order_id, created_at, status::text, restaurant_id,
		          (SELECT name FROM restaurants WHERE restaurant_id = orders.restaurant_id) AS restaurant_name,
		          subtotal::text, taxes::text, delivery_fee::text, discount::text, total_amount::text,
			  payment_method, address_id, instructions`, in.UserID, *lockedCart.RestaurantID, in.AddressID,
		minorDecimal(quote.Subtotal), minorDecimal(quote.Taxes), minorDecimal(quote.DeliveryFee), minorDecimal(quote.Discount), minorDecimal(quote.TotalAmount), strings.TrimSpace(in.PaymentMethod), in.IdempotencyKey, strings.TrimSpace(in.Instructions)); err != nil {
		return ordermodels.Order{}, false, err
	}
	for _, item := range lockedCart.Items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO order_items (order_id, order_created_at, item_id, item_name_snapshot, item_price_snapshot, quantity, line_total, customisations)
			VALUES ($1, $2, $3, $4, $5::decimal, $6, $7::decimal, $8::jsonb)`, row.OrderID, row.CreatedAt, item.ItemID, item.Name, minorDecimal(item.UnitPrice), item.Quantity, minorDecimal(item.LineTotal), mustJSON(item.Customisations)); err != nil {
			return ordermodels.Order{}, false, err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE carts SET status = 'converted', restaurant_id = NULL, updated_at = NOW() WHERE cart_id = $1 AND user_id = $2 AND status = 'active' AND (expires_at IS NULL OR expires_at > NOW())`, lockedCart.CartID, in.UserID)
	if err != nil {
		return ordermodels.Order{}, false, err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return ordermodels.Order{}, false, ErrCartNotActive
	}
	if err := tx.Commit(); err != nil {
		return ordermodels.Order{}, false, err
	}
	out, err := r.mapOrder(ctx, nil, row)
	return out, false, err
}

func (r *PostgresRepository) lockActiveCart(ctx context.Context, tx *sqlx.Tx, userID, cartID uuid.UUID) (cartmodels.Cart, error) {
	if cartID == uuid.Nil {
		return cartmodels.Cart{}, ErrCartNotActive
	}
	var meta struct {
		CartID       uuid.UUID      `db:"cart_id"`
		RestaurantID *uuid.UUID     `db:"restaurant_id"`
		Restaurant   sql.NullString `db:"restaurant_name"`
	}
	if err := tx.GetContext(ctx, &meta, `
		SELECT c.cart_id, c.restaurant_id, r.name AS restaurant_name
		FROM carts c LEFT JOIN restaurants r ON r.restaurant_id = c.restaurant_id
		WHERE c.cart_id = $1 AND c.user_id = $2 AND c.status = 'active'
		  AND (c.expires_at IS NULL OR c.expires_at > NOW())
		FOR UPDATE OF c`, cartID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return cartmodels.Cart{}, ErrCartNotActive
		}
		return cartmodels.Cart{}, err
	}
	var rows []struct {
		CartItemID     uuid.UUID       `db:"cart_item_id"`
		ItemID         uuid.UUID       `db:"item_id"`
		Name           string          `db:"name"`
		Price          string          `db:"price"`
		Quantity       int             `db:"quantity"`
		Customisations json.RawMessage `db:"customisations"`
	}
	if err := tx.SelectContext(ctx, &rows, `
		SELECT ci.cart_item_id, ci.item_id, i.name, i.price::text, ci.quantity, ci.customisations
		FROM cart_items ci JOIN menu_items i ON i.item_id = ci.item_id
		WHERE ci.cart_id = $1 ORDER BY ci.added_at, ci.cart_item_id`, cartID); err != nil {
		return cartmodels.Cart{}, err
	}
	out := cartmodels.Cart{CartID: meta.CartID, RestaurantID: meta.RestaurantID, RestaurantName: nullString(meta.Restaurant), Items: make([]cartmodels.Item, 0, len(rows)), Currency: "INR"}
	for _, row := range rows {
		unit, err := decimalToMinor(row.Price)
		if err != nil {
			return cartmodels.Cart{}, err
		}
		customisations := []string{}
		if len(row.Customisations) > 0 && string(row.Customisations) != "null" {
			if err := json.Unmarshal(row.Customisations, &customisations); err != nil {
				return cartmodels.Cart{}, err
			}
		}
		lineTotal, ok := orderSafeMultiply(unit, int64(row.Quantity))
		if !ok {
			return cartmodels.Cart{}, fmt.Errorf("cart item total overflow")
		}
		out.Items = append(out.Items, cartmodels.Item{CartItemID: row.CartItemID, ItemID: row.ItemID, Name: row.Name, Quantity: row.Quantity, UnitPrice: unit, LineTotal: lineTotal, Customisations: customisations})
		if lineTotal > int64(^uint64(0)>>1)-out.Subtotal {
			return cartmodels.Cart{}, fmt.Errorf("cart subtotal overflow")
		}
		out.Subtotal += lineTotal
	}
	return out, nil
}

func orderSafeMultiply(a, b int64) (int64, bool) {
	if a < 0 || b < 0 || (b != 0 && a > (int64(^uint64(0)>>1))/b) {
		return 0, false
	}
	return a * b, true
}

func mustJSON(value []string) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func (r *PostgresRepository) validateAddress(ctx context.Context, userID, addressID, restaurantID uuid.UUID) error {
	var address addressRow
	if err := r.db.GetContext(ctx, &address, `SELECT address_id, latitude, longitude FROM addresses WHERE address_id = $1 AND user_id = $2 AND is_deleted = FALSE`, addressID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAddressNotFound
		}
		return err
	}
	if !address.Latitude.Valid || !address.Longitude.Valid {
		return ErrNotServiceable
	}
	var serviceable bool
	if err := r.db.GetContext(ctx, &serviceable, `SELECT ST_DWithin(r.location::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, r.service_radius_km * 1000) FROM restaurants r WHERE r.restaurant_id = $3 AND r.status = 'active'`, address.Longitude.Float64, address.Latitude.Float64, restaurantID); err != nil {
		return err
	}
	if !serviceable {
		return ErrNotServiceable
	}
	return nil
}

func (r *PostgresRepository) validateAddressTx(ctx context.Context, tx *sqlx.Tx, userID, addressID, restaurantID uuid.UUID) error {
	var address addressRow
	if err := tx.GetContext(ctx, &address, `SELECT address_id, latitude, longitude FROM addresses WHERE address_id = $1 AND user_id = $2 AND is_deleted = FALSE FOR SHARE`, addressID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAddressNotFound
		}
		return err
	}
	if !address.Latitude.Valid || !address.Longitude.Valid {
		return ErrNotServiceable
	}
	var serviceable bool
	if err := tx.GetContext(ctx, &serviceable, `SELECT ST_DWithin(r.location::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, r.service_radius_km * 1000) FROM restaurants r WHERE r.restaurant_id = $3 AND r.status = 'active'`, address.Longitude.Float64, address.Latitude.Float64, restaurantID); err != nil {
		return err
	}
	if !serviceable {
		return ErrNotServiceable
	}
	return nil
}

func (r *PostgresRepository) validateItems(ctx context.Context, restaurantID uuid.UUID, items []cartmodels.Item) error {
	for _, item := range items {
		var row itemRow
		if err := r.db.GetContext(ctx, &row, `SELECT item_id, restaurant_id, name, price::text, is_available FROM menu_items WHERE item_id = $1 AND restaurant_id = $2 AND is_deleted = FALSE`, item.ItemID, restaurantID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrItemUnavailable
			}
			return err
		}
		if !row.Available {
			return ErrItemUnavailable
		}
		current, err := decimalToMinor(row.Price)
		if err != nil {
			return err
		}
		if current != item.UnitPrice {
			return ErrItemPriceChanged
		}
	}
	return nil
}

func (r *PostgresRepository) validateItemsTx(ctx context.Context, tx *sqlx.Tx, restaurantID uuid.UUID, items []cartmodels.Item) error {
	for _, item := range items {
		var row itemRow
		if err := tx.GetContext(ctx, &row, `SELECT item_id, restaurant_id, name, price::text, is_available FROM menu_items WHERE item_id = $1 AND restaurant_id = $2 AND is_deleted = FALSE FOR UPDATE`, item.ItemID, restaurantID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrItemUnavailable
			}
			return err
		}
		if !row.Available {
			return ErrItemUnavailable
		}
		current, err := decimalToMinor(row.Price)
		if err != nil {
			return err
		}
		if current != item.UnitPrice {
			return ErrItemPriceChanged
		}
	}
	return nil
}

func quoteFor(subtotal int64, now time.Time) ordermodels.Quote {
	taxes := subtotal * ordermodels.TaxRatePercent / 100
	return ordermodels.Quote{Subtotal: subtotal, Taxes: taxes, DeliveryFee: ordermodels.DeliveryFeeMinor, Discount: 0, TotalAmount: subtotal + taxes + ordermodels.DeliveryFeeMinor, Currency: "INR", EstimatedDeliveryAt: now.Add(ordermodels.EstimatedDeliveryMin * time.Minute)}
}

func minorDecimal(value int64) string { return fmt.Sprintf("%d.%02d", value/100, value%100) }

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

func (r *PostgresRepository) mapOrder(ctx context.Context, tx *sqlx.Tx, row orderRow) (ordermodels.Order, error) {
	out := ordermodels.Order{OrderID: row.OrderID, Status: row.Status, RestaurantID: row.RestaurantID, RestaurantName: row.Restaurant, Subtotal: mustMinor(row.Subtotal), Taxes: mustMinor(row.Taxes), DeliveryFee: mustMinor(row.DeliveryFee), Discount: mustMinor(row.Discount), TotalAmount: mustMinor(row.TotalAmount), Currency: "INR", PaymentMethod: row.PaymentMethod, AddressID: row.AddressID, Instructions: nullString(row.Instructions), CreatedAt: row.CreatedAt, EstimatedDelivery: row.CreatedAt.Add(ordermodels.EstimatedDeliveryMin * time.Minute), Items: []cartmodels.Item{}}
	var rows []struct {
		ItemID         uuid.UUID       `db:"item_id"`
		Name           string          `db:"item_name_snapshot"`
		Price          string          `db:"item_price_snapshot"`
		Quantity       int             `db:"quantity"`
		LineTotal      string          `db:"line_total"`
		Customisations json.RawMessage `db:"customisations"`
	}
	var err error
	if tx != nil {
		err = tx.SelectContext(ctx, &rows, `SELECT item_id, item_name_snapshot, item_price_snapshot::text, quantity, line_total::text, customisations FROM order_items WHERE order_id = $1 AND order_created_at = $2 ORDER BY order_item_id`, row.OrderID, row.CreatedAt)
	} else {
		err = r.db.SelectContext(ctx, &rows, `SELECT item_id, item_name_snapshot, item_price_snapshot::text, quantity, line_total::text, customisations FROM order_items WHERE order_id = $1 AND order_created_at = $2 ORDER BY order_item_id`, row.OrderID, row.CreatedAt)
	}
	if err != nil {
		return ordermodels.Order{}, err
	}
	for _, item := range rows {
		customisations := []string{}
		if len(item.Customisations) > 0 && string(item.Customisations) != "null" {
			_ = json.Unmarshal(item.Customisations, &customisations)
		}
		out.Items = append(out.Items, cartmodels.Item{ItemID: item.ItemID, Name: item.Name, UnitPrice: mustMinor(item.Price), Quantity: item.Quantity, LineTotal: mustMinor(item.LineTotal), Customisations: customisations})
	}
	return out, nil
}

func mustMinor(raw string) int64 { value, _ := decimalToMinor(raw); return value }
func nullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
