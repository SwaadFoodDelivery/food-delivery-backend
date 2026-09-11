package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
		       o.total_amount::text, o.payment_method, o.address_id, o.instructions, r.delivery_time_min
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
	OrderID         uuid.UUID      `db:"order_id"`
	CreatedAt       time.Time      `db:"created_at"`
	Status          string         `db:"status"`
	RestaurantID    uuid.UUID      `db:"restaurant_id"`
	Restaurant      string         `db:"restaurant_name"`
	Subtotal        string         `db:"subtotal"`
	Taxes           string         `db:"taxes"`
	DeliveryFee     string         `db:"delivery_fee"`
	Discount        string         `db:"discount"`
	TotalAmount     string         `db:"total_amount"`
	PaymentMethod   string         `db:"payment_method"`
	AddressID       uuid.UUID      `db:"address_id"`
	Instructions    sql.NullString `db:"instructions"`
	DeliveryTimeMin int            `db:"delivery_time_min"`
}

type historyItemRow struct {
	OrderID        uuid.UUID      `db:"order_id"`
	Status         string         `db:"status"`
	Restaurant     string         `db:"restaurant_name"`
	TotalAmount    string         `db:"total_amount"`
	PaymentMethod  string         `db:"payment_method"`
	DeliveryStatus sql.NullString `db:"delivery_status"`
	CreatedAt      time.Time      `db:"created_at"`
}

type statusEventRow struct {
	FromStatus sql.NullString `db:"from_status"`
	ToStatus   string         `db:"to_status"`
	ChangedAt  time.Time      `db:"changed_at"`
}

func (r *PostgresRepository) ListForUser(ctx context.Context, userID uuid.UUID, limit int) ([]ordermodels.HistoryItem, error) {
	rows := make([]historyItemRow, 0)
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT o.order_id, o.status::text, r.name AS restaurant_name, o.total_amount::text,
		       o.payment_method, d.status::text AS delivery_status, o.created_at
		FROM orders o
		JOIN restaurants r ON r.restaurant_id = o.restaurant_id
		LEFT JOIN deliveries d ON d.order_id = o.order_id AND d.order_created_at = o.created_at
		WHERE o.user_id = $1
		ORDER BY o.created_at DESC
		LIMIT $2`, userID, limit); err != nil {
		return nil, err
	}
	out := make([]ordermodels.HistoryItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, ordermodels.HistoryItem{OrderID: row.OrderID, Status: row.Status, RestaurantName: row.Restaurant, TotalAmount: mustMinor(row.TotalAmount), Currency: "INR", PaymentMethod: row.PaymentMethod, DeliveryStatus: nullString(row.DeliveryStatus), CreatedAt: row.CreatedAt})
	}
	return out, nil
}

func (r *PostgresRepository) GetHistory(ctx context.Context, userID, orderID uuid.UUID) (ordermodels.History, error) {
	var current struct {
		CreatedAt time.Time `db:"created_at"`
		Status    string    `db:"status"`
	}
	if err := r.db.GetContext(ctx, &current, `SELECT created_at,status::text FROM orders WHERE order_id = $1 AND user_id = $2 ORDER BY created_at DESC LIMIT 1`, orderID, userID); errors.Is(err, sql.ErrNoRows) {
		return ordermodels.History{}, ErrOrderNotFound
	} else if err != nil {
		return ordermodels.History{}, err
	}
	createdAt := current.CreatedAt
	var orderRows []statusEventRow
	if err := r.db.SelectContext(ctx, &orderRows, `
		SELECT from_status::text, to_status::text, changed_at
		FROM order_status_history
		WHERE order_id = $1 AND order_created_at = $2
		ORDER BY changed_at`, orderID, createdAt); err != nil {
		return ordermodels.History{}, err
	}
	var deliveryID uuid.UUID
	var deliveryRows []statusEventRow
	if err := r.db.GetContext(ctx, &deliveryID, `SELECT delivery_id FROM deliveries WHERE order_id = $1 AND order_created_at = $2`, orderID, createdAt); err == nil {
		if err := r.db.SelectContext(ctx, &deliveryRows, `
			SELECT from_status::text, to_status::text, changed_at
			FROM delivery_status_history WHERE delivery_id = $1 ORDER BY changed_at`, deliveryID); err != nil {
			return ordermodels.History{}, err
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ordermodels.History{}, err
	}
	return ordermodels.History{OrderID: orderID, Status: current.Status, OrderStatus: mapEvents(orderRows), DeliveryStatus: mapEvents(deliveryRows)}, nil
}

func (r *PostgresRepository) CancelForUser(ctx context.Context, userID, orderID uuid.UUID) (ordermodels.HistoryItem, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return ordermodels.HistoryItem{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var row historyItemRow
	if err := tx.GetContext(ctx, &row, `
		SELECT o.order_id, o.status::text, r.name AS restaurant_name, o.total_amount::text,
		       o.payment_method, d.status::text AS delivery_status, o.created_at
		FROM orders o JOIN restaurants r ON r.restaurant_id = o.restaurant_id
		LEFT JOIN deliveries d ON d.order_id = o.order_id AND d.order_created_at = o.created_at
		WHERE o.order_id = $1 AND o.user_id = $2
		FOR UPDATE OF o`, orderID, userID); errors.Is(err, sql.ErrNoRows) {
		return ordermodels.HistoryItem{}, ErrOrderNotFound
	} else if err != nil {
		return ordermodels.HistoryItem{}, err
	}
	if row.Status == "cancelled" || row.Status == "rejected" || row.Status == "delivered" {
		return ordermodels.HistoryItem{}, ErrOrderNotCancelable
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET status = 'cancelled', updated_by = $1, updated_at = NOW() WHERE order_id = $2 AND user_id = $1`, userID, orderID); err != nil {
		return ordermodels.HistoryItem{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE deliveries SET next_transition_at = NULL, updated_by = $1, updated_at = NOW() WHERE order_id = $2 AND order_created_at = $3 AND status <> 'delivered'`, userID, orderID, row.CreatedAt); err != nil {
		return ordermodels.HistoryItem{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO notifications (recipient_id, recipient_type, channels, title, body, status)
		VALUES ($1, 'client', ARRAY['in_app'], 'Order cancelled', 'Your demo order was cancelled successfully.', 'queued')`, userID); err != nil {
		return ordermodels.HistoryItem{}, err
	}
	beforeJSON := fmt.Sprintf(`{"status":%q}`, row.Status)
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_logs (actor_id, actor_role, action, entity_type, entity_id, before, after) VALUES ($1, 'client', 'order_cancelled', 'order', $2, $3::jsonb, '{"status":"cancelled"}'::jsonb)`, userID, orderID.String(), beforeJSON); err != nil {
		return ordermodels.HistoryItem{}, err
	}
	if err := tx.Commit(); err != nil {
		return ordermodels.HistoryItem{}, err
	}
	row.Status = "cancelled"
	row.DeliveryStatus = sql.NullString{}
	return ordermodels.HistoryItem{OrderID: row.OrderID, Status: row.Status, RestaurantName: row.Restaurant, TotalAmount: mustMinor(row.TotalAmount), Currency: "INR", PaymentMethod: row.PaymentMethod, CreatedAt: row.CreatedAt}, nil
}

func mapEvents(rows []statusEventRow) []ordermodels.StatusEvent {
	out := make([]ordermodels.StatusEvent, 0, len(rows))
	for _, row := range rows {
		from := ""
		if row.FromStatus.Valid {
			from = row.FromStatus.String
		}
		out = append(out, ordermodels.StatusEvent{FromStatus: from, ToStatus: row.ToStatus, ChangedAt: row.ChangedAt})
	}
	return out
}

func (r *PostgresRepository) Quote(ctx context.Context, userID uuid.UUID, cart cartmodels.Cart, addressID uuid.UUID) (ordermodels.Quote, error) {
	if len(cart.Items) == 0 || cart.RestaurantID == nil {
		return ordermodels.Quote{}, ErrCartEmpty
	}
	serviceability, err := r.CheckServiceability(ctx, userID, addressID, *cart.RestaurantID)
	if err != nil {
		return ordermodels.Quote{}, err
	}
	if !serviceability.Serviceable {
		return ordermodels.Quote{}, ErrNotServiceable
	}
	if err := r.validateItems(ctx, *cart.RestaurantID, cart.Items); err != nil {
		return ordermodels.Quote{}, err
	}
	return quoteFor(cart.Subtotal, serviceability, time.Now().UTC()), nil
}

type serviceabilityRow struct {
	DistanceKM      float64 `db:"distance_km"`
	WithinRadius    bool    `db:"within_radius"`
	ServiceRadiusKM float64 `db:"service_radius_km"`
	IsOpen          bool    `db:"is_open"`
	Status          string  `db:"status"`
	DeliveryTimeMin int     `db:"delivery_time_min"`
}

func (r *PostgresRepository) CheckServiceability(ctx context.Context, userID, addressID, restaurantID uuid.UUID) (ordermodels.Serviceability, error) {
	return r.checkServiceabilityWithQueryer(ctx, r.db, userID, addressID, restaurantID)
}

func (r *PostgresRepository) checkServiceabilityWithQueryer(ctx context.Context, queryer sqlx.QueryerContext, userID, addressID, restaurantID uuid.UUID) (ordermodels.Serviceability, error) {
	var address addressRow
	if err := sqlx.GetContext(ctx, queryer, &address, `SELECT address_id, latitude, longitude FROM addresses WHERE address_id = $1 AND user_id = $2 AND is_deleted = FALSE`, addressID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ordermodels.Serviceability{}, ErrAddressNotFound
		}
		return ordermodels.Serviceability{}, err
	}
	if !address.Latitude.Valid || !address.Longitude.Valid {
		return ordermodels.Serviceability{Serviceable: false, ReasonCode: "missing_coordinates", Reason: "Add a map location to this address before ordering.", Currency: "INR"}, nil
	}
	var row serviceabilityRow
	if err := sqlx.GetContext(ctx, queryer, &row, `
		SELECT ST_Distance(r.location::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) / 1000.0 AS distance_km,
		       ST_DWithin(r.location::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, r.service_radius_km * 1000) AS within_radius,
		       r.service_radius_km, r.is_open, r.status::text, r.delivery_time_min
		FROM restaurants r WHERE r.restaurant_id = $3
	`, address.Longitude.Float64, address.Latitude.Float64, restaurantID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ordermodels.Serviceability{}, ErrRestaurantNotFound
		}
		return ordermodels.Serviceability{}, err
	}
	decision := ordermodels.Serviceability{DistanceKM: math.Round(row.DistanceKM*100) / 100, ServiceRadiusKM: row.ServiceRadiusKM, EstimatedDeliveryMin: row.DeliveryTimeMin, Currency: "INR"}
	if decision.EstimatedDeliveryMin <= 0 {
		decision.EstimatedDeliveryMin = ordermodels.EstimatedDeliveryMin
	}
	if row.Status != "active" {
		decision.ReasonCode, decision.Reason = "restaurant_unavailable", "This restaurant is not accepting orders right now."
		return decision, nil
	}
	if !row.IsOpen {
		decision.ReasonCode, decision.Reason = "restaurant_closed", "This restaurant is currently closed."
		return decision, nil
	}
	if !row.WithinRadius {
		decision.ReasonCode, decision.Reason = "outside_delivery_radius", "This address is outside the restaurant's delivery area."
		return decision, nil
	}
	decision.Serviceable = true
	decision.ReasonCode, decision.Reason = "serviceable", "This address is serviceable."
	decision.DeliveryFee = deliveryFeeForDistance(decision.DistanceKM)
	return decision, nil
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
		       o.total_amount::text, o.payment_method, o.address_id, o.instructions, r.delivery_time_min
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
	serviceability, err := r.checkServiceabilityWithQueryer(ctx, tx, in.UserID, in.AddressID, *lockedCart.RestaurantID)
	if err != nil {
		return ordermodels.Order{}, false, err
	}
	if !serviceability.Serviceable {
		return ordermodels.Order{}, false, ErrNotServiceable
	}
	quote = quoteFor(lockedCart.Subtotal, serviceability, time.Now().UTC())
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
			  payment_method, address_id, instructions,
			          (SELECT delivery_time_min FROM restaurants WHERE restaurant_id = orders.restaurant_id) AS delivery_time_min`, in.UserID, *lockedCart.RestaurantID, in.AddressID,
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
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO notifications (recipient_id, recipient_type, channels, title, body, status)
		VALUES ($1, 'client', ARRAY['in_app'], 'Order placed', 'Your Shamgarh demo order is now being prepared.', 'queued')`, in.UserID); err != nil {
		return ordermodels.Order{}, false, err
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
	decision, err := r.CheckServiceability(ctx, userID, addressID, restaurantID)
	if err != nil {
		return err
	}
	if !decision.Serviceable {
		return ErrNotServiceable
	}
	return nil
}

func (r *PostgresRepository) validateAddressTx(ctx context.Context, tx *sqlx.Tx, userID, addressID, restaurantID uuid.UUID) error {
	decision, err := r.checkServiceabilityWithQueryer(ctx, tx, userID, addressID, restaurantID)
	if err != nil {
		return err
	}
	if !decision.Serviceable {
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

func quoteFor(subtotal int64, serviceability ordermodels.Serviceability, now time.Time) ordermodels.Quote {
	taxes := subtotal * ordermodels.TaxRatePercent / 100
	return ordermodels.Quote{Subtotal: subtotal, Taxes: taxes, DeliveryFee: serviceability.DeliveryFee, Discount: 0, TotalAmount: subtotal + taxes + serviceability.DeliveryFee, Currency: "INR", EstimatedDeliveryAt: now.Add(time.Duration(serviceability.EstimatedDeliveryMin) * time.Minute), Serviceability: serviceability}
}

func deliveryFeeForDistance(distanceKM float64) int64 {
	fee := ordermodels.DeliveryFeeMinor
	if distanceKM > 3 {
		fee += int64(math.Ceil(distanceKM-3)) * 500
	}
	if fee > 8000 {
		return 8000
	}
	return fee
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
	etaMinutes := row.DeliveryTimeMin
	if etaMinutes <= 0 {
		etaMinutes = ordermodels.EstimatedDeliveryMin
	}
	out := ordermodels.Order{OrderID: row.OrderID, Status: row.Status, RestaurantID: row.RestaurantID, RestaurantName: row.Restaurant, Subtotal: mustMinor(row.Subtotal), Taxes: mustMinor(row.Taxes), DeliveryFee: mustMinor(row.DeliveryFee), Discount: mustMinor(row.Discount), TotalAmount: mustMinor(row.TotalAmount), Currency: "INR", PaymentMethod: row.PaymentMethod, AddressID: row.AddressID, Instructions: nullString(row.Instructions), CreatedAt: row.CreatedAt, EstimatedDelivery: row.CreatedAt.Add(time.Duration(etaMinutes) * time.Minute), Items: []cartmodels.Item{}}
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
