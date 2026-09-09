package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"food-delivery-backend/internal/services/operations/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type PostgresRepository struct{ db *sqlx.DB }

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository { return &PostgresRepository{db: db} }

type orderRow struct {
	OrderID        uuid.UUID      `db:"order_id"`
	Status         string         `db:"status"`
	TotalAmount    string         `db:"total_amount"`
	PaymentMethod  string         `db:"payment_method"`
	CustomerName   string         `db:"customer_name"`
	RestaurantName string         `db:"restaurant_name"`
	DeliveryStatus sql.NullString `db:"delivery_status"`
	PartnerName    sql.NullString `db:"partner_name"`
	CreatedAt      sql.NullTime   `db:"created_at"`
	UpdatedAt      sql.NullTime   `db:"updated_at"`
}

type driverRow struct {
	DriverID       uuid.UUID      `db:"driver_id"`
	Name           string         `db:"name"`
	IsAvailable    bool           `db:"is_available"`
	CurrentCity    sql.NullString `db:"current_city"`
	ActiveOrderID  sql.NullString `db:"active_order_id"`
	DeliveryStatus sql.NullString `db:"delivery_status"`
}

func (r *PostgresRepository) GetOverview(ctx context.Context, status string) (models.Overview, error) {
	var rows []orderRow
	query := `
		SELECT o.order_id, o.status::text, o.total_amount::text, o.payment_method,
		       customer.name AS customer_name, restaurant.name AS restaurant_name,
		       d.status::text AS delivery_status, partner.name AS partner_name,
		       o.created_at, o.updated_at
		FROM orders o
		JOIN users customer ON customer.user_id = o.user_id
		JOIN restaurants restaurant ON restaurant.restaurant_id = o.restaurant_id
		LEFT JOIN deliveries d ON d.order_id = o.order_id AND d.order_created_at = o.created_at
		LEFT JOIN users partner ON partner.user_id = d.partner_id
		WHERE ($1 = '' OR o.status::text = $1)
		ORDER BY o.created_at DESC
		LIMIT 50`
	if err := r.db.SelectContext(ctx, &rows, query, status); err != nil {
		return models.Overview{}, err
	}

	var driverRows []driverRow
	if err := r.db.SelectContext(ctx, &driverRows, `
		SELECT u.user_id AS driver_id, u.name, dp.is_available, dp.current_city,
		       active.order_id::text AS active_order_id, active.status AS delivery_status
		FROM users u
		JOIN driver_profiles dp ON dp.user_id = u.user_id
		LEFT JOIN LATERAL (
			SELECT d.order_id, d.status::text AS status
			FROM deliveries d
			JOIN orders o ON o.order_id = d.order_id AND o.created_at = d.order_created_at
			WHERE d.partner_id = u.user_id AND d.status <> 'delivered'
			  AND o.status NOT IN ('cancelled', 'rejected', 'delivered')
			ORDER BY d.updated_at DESC
			LIMIT 1
		) active ON TRUE
		WHERE u.role = 'driver' AND u.account_status = 'active' AND u.is_deleted = FALSE
		ORDER BY u.name, u.user_id`, &driverRows); err != nil {
		return models.Overview{}, err
	}

	var total, active, delivered, failedPayments, stalledDeliveries int
	if err := r.db.GetContext(ctx, &total, `SELECT count(*) FROM orders`); err != nil {
		return models.Overview{}, err
	}
	if err := r.db.GetContext(ctx, &active, `SELECT count(*) FROM orders WHERE status NOT IN ('cancelled', 'rejected', 'delivered')`); err != nil {
		return models.Overview{}, err
	}
	if err := r.db.GetContext(ctx, &delivered, `SELECT count(*) FROM orders WHERE status = 'delivered'`); err != nil {
		return models.Overview{}, err
	}
	if err := r.db.GetContext(ctx, &failedPayments, `SELECT count(*) FROM payments WHERE status = 'failed'`); err != nil {
		return models.Overview{}, err
	}
	if err := r.db.GetContext(ctx, &stalledDeliveries, `
		SELECT count(*) FROM deliveries d JOIN orders o ON o.order_id = d.order_id AND o.created_at = d.order_created_at
		WHERE d.next_transition_at IS NOT NULL AND d.next_transition_at < NOW()
		  AND d.status <> 'delivered' AND o.status NOT IN ('cancelled', 'rejected', 'delivered')`); err != nil {
		return models.Overview{}, err
	}

	drivers := make([]models.Driver, 0, len(driverRows))
	available, activeDrivers := 0, 0
	for _, row := range driverRows {
		if row.IsAvailable {
			available++
		}
		if row.ActiveOrderID.Valid {
			activeDrivers++
		}
		drivers = append(drivers, mapDriver(row))
	}

	orders := make([]models.Order, 0, len(rows))
	for _, row := range rows {
		orders = append(orders, mapOrder(row))
	}
	return models.Overview{
		Summary: models.Summary{TotalOrders: total, ActiveOrders: active, DeliveredOrders: delivered, AvailableDrivers: available, ActiveDrivers: activeDrivers, FailedPayments: failedPayments, StalledDeliveries: stalledDeliveries},
		Orders:  orders,
		Drivers: drivers,
	}, nil
}

func (r *PostgresRepository) CancelOrder(ctx context.Context, actorID, orderID uuid.UUID) (models.Order, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return models.Order{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var before orderRow
	if err := tx.GetContext(ctx, &before, `
		SELECT o.order_id, o.status::text, o.total_amount::text, o.payment_method,
		       customer.name AS customer_name, restaurant.name AS restaurant_name,
		       d.status::text AS delivery_status, partner.name AS partner_name,
		       o.created_at, o.updated_at
		FROM orders o
		JOIN users customer ON customer.user_id = o.user_id
		JOIN restaurants restaurant ON restaurant.restaurant_id = o.restaurant_id
		LEFT JOIN deliveries d ON d.order_id = o.order_id AND d.order_created_at = o.created_at
		LEFT JOIN users partner ON partner.user_id = d.partner_id
		WHERE o.order_id = $1
		ORDER BY o.created_at DESC
		LIMIT 1 FOR UPDATE OF o`, orderID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Order{}, ErrOrderNotFound
		}
		return models.Order{}, err
	}
	if before.Status == "cancelled" || before.Status == "rejected" || before.Status == "delivered" {
		return models.Order{}, ErrOrderNotCancelable
	}

	beforeJSON, _ := json.Marshal(map[string]string{"status": before.Status})
	if _, err := tx.ExecContext(ctx, `
		UPDATE orders SET status = 'cancelled', updated_by = $1, updated_at = NOW()
		WHERE order_id = $2 AND created_at = $3`, actorID, orderID, before.CreatedAt.Time); err != nil {
		return models.Order{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE deliveries SET next_transition_at = NULL, updated_by = $1, updated_at = NOW()
		WHERE order_id = $2 AND order_created_at = $3 AND status <> 'delivered'`, actorID, orderID, before.CreatedAt.Time); err != nil {
		return models.Order{}, err
	}
	afterJSON, _ := json.Marshal(map[string]string{"status": "cancelled"})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (actor_id, actor_role, action, entity_type, entity_id, before, after)
		VALUES ($1, 'restaurant_manager', 'ops_order_cancelled', 'order', $2, $3::jsonb, $4::jsonb)`, actorID, orderID.String(), beforeJSON, afterJSON); err != nil {
		return models.Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.Order{}, err
	}
	before.Status = "cancelled"
	return mapOrder(before), nil
}

func mapOrder(row orderRow) models.Order {
	deliveryStatus := nullString(row.DeliveryStatus)
	if row.Status == "cancelled" || row.Status == "rejected" {
		deliveryStatus = ""
	}
	return models.Order{OrderID: row.OrderID, Status: row.Status, TotalAmount: minorAmount(row.TotalAmount), Currency: "INR", PaymentMethod: row.PaymentMethod, CustomerName: row.CustomerName, RestaurantName: row.RestaurantName, DeliveryStatus: deliveryStatus, PartnerName: nullString(row.PartnerName), CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
}

func mapDriver(row driverRow) models.Driver {
	var orderID *uuid.UUID
	if row.ActiveOrderID.Valid {
		parsed, err := uuid.Parse(row.ActiveOrderID.String)
		if err == nil {
			orderID = &parsed
		}
	}
	return models.Driver{DriverID: row.DriverID, Name: row.Name, IsAvailable: row.IsAvailable, CurrentCity: nullString(row.CurrentCity), ActiveOrderID: orderID, DeliveryStatus: nullString(row.DeliveryStatus)}
}

func nullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func minorAmount(raw string) int64 {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return int64(value*100 + 0.5)
}

func ValidateStatus(status string) error {
	if status == "" {
		return nil
	}
	switch status {
	case "order_created", "pending_payment", "confirmed", "accepted", "preparing", "ready_for_pickup", "picked_up", "out_for_delivery", "delivered", "cancelled", "rejected":
		return nil
	default:
		return fmt.Errorf("unsupported order status %q", status)
	}
}
