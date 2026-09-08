package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"food-delivery-backend/internal/services/delivery/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type PostgresRepository struct{ db *sqlx.DB }

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository { return &PostgresRepository{db: db} }

type deliveryRow struct {
	DeliveryID       uuid.UUID  `db:"delivery_id"`
	OrderID          uuid.UUID  `db:"order_id"`
	Provider         string     `db:"provider"`
	Status           string     `db:"status"`
	PartnerID        uuid.UUID  `db:"partner_id"`
	PartnerName      string     `db:"partner_name"`
	PartnerPhone     string     `db:"partner_phone"`
	AssignedAt       time.Time  `db:"assigned_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
	NextTransitionAt *time.Time `db:"next_transition_at"`
}

func (r *PostgresRepository) EnsureMockDelivery(ctx context.Context, orderID uuid.UUID, createdAt time.Time, duration time.Duration) (models.Delivery, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return models.Delivery{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var row deliveryRow
	err = tx.GetContext(ctx, &row, deliverySelect+` WHERE d.order_id = $1 AND d.order_created_at = $2 FOR UPDATE`, orderID, createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		var partnerID uuid.UUID
		err = tx.GetContext(ctx, &partnerID, `
			SELECT u.user_id
			FROM users u
			JOIN driver_profiles dp ON dp.user_id = u.user_id
			WHERE u.role = 'driver' AND u.account_status = 'active' AND u.is_deleted = FALSE
			  AND dp.is_available = TRUE
			ORDER BY u.user_id
			LIMIT 1
			FOR UPDATE OF u`)
		if errors.Is(err, sql.ErrNoRows) {
			return models.Delivery{}, ErrNoDemoPartner
		}
		if err != nil {
			return models.Delivery{}, err
		}
		stepSeconds := int64(duration / time.Second / 5)
		if stepSeconds < 1 {
			stepSeconds = 1
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO deliveries (order_id, order_created_at, partner_id, provider, status, assigned_at, updated_at, next_transition_at)
			VALUES ($1, $2, $3, 'mock', 'assigned', NOW(), NOW(), NOW() + ($4 * INTERVAL '1 second'))
			ON CONFLICT (order_id, order_created_at) DO NOTHING`, orderID, createdAt, partnerID, stepSeconds)
		if err != nil {
			return models.Delivery{}, err
		}
		if err = tx.GetContext(ctx, &row, deliverySelect+` WHERE d.order_id = $1 AND d.order_created_at = $2 FOR UPDATE`, orderID, createdAt); err != nil {
			return models.Delivery{}, err
		}
	} else if err != nil {
		return models.Delivery{}, err
	}

	if _, err = tx.ExecContext(ctx, `
		UPDATE orders SET status = 'confirmed', updated_at = NOW()
		WHERE order_id = $1 AND created_at = $2 AND status NOT IN ('cancelled', 'rejected', 'delivered')`, orderID, createdAt); err != nil {
		return models.Delivery{}, err
	}
	if err = tx.Commit(); err != nil {
		return models.Delivery{}, err
	}
	return mapDelivery(row), nil
}

func (r *PostgresRepository) AdvanceMockDeliveries(ctx context.Context, now time.Time, duration time.Duration) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var rows []struct {
		DeliveryID       uuid.UUID  `db:"delivery_id"`
		OrderID          uuid.UUID  `db:"order_id"`
		OrderCreatedAt   time.Time  `db:"order_created_at"`
		Status           string     `db:"status"`
		NextTransitionAt *time.Time `db:"next_transition_at"`
	}
	if err := tx.SelectContext(ctx, &rows, `
		SELECT d.delivery_id, d.order_id, d.order_created_at, d.status::text, d.next_transition_at
		FROM deliveries d
		JOIN orders o ON o.order_id = d.order_id AND o.created_at = d.order_created_at
		WHERE d.provider = 'mock' AND d.next_transition_at IS NOT NULL AND d.next_transition_at <= $1
		  AND o.status NOT IN ('cancelled', 'rejected')
		FOR UPDATE OF d, o SKIP LOCKED`, now); err != nil {
		return err
	}

	step := duration / 5
	if step < time.Second {
		step = time.Second
	}
	for _, row := range rows {
		status := row.Status
		due := row.NextTransitionAt
		for due != nil && !due.After(now) && status != models.StatusDelivered {
			next := nextStatus(status)
			if next == "" {
				break
			}
			var nextDue *time.Time
			if next != models.StatusDelivered {
				n := due.Add(step)
				nextDue = &n
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE deliveries SET status = $1::delivery_status, updated_at = NOW(), next_transition_at = $2
				WHERE delivery_id = $3`, next, nextDue, row.DeliveryID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE orders SET status = $1::order_status, updated_at = NOW()
				WHERE order_id = $2 AND created_at = $3 AND status NOT IN ('cancelled', 'rejected')`, orderStatusForDelivery(next), row.OrderID, row.OrderCreatedAt); err != nil {
				return err
			}
			status, due = next, nextDue
		}
	}
	return tx.Commit()
}

func (r *PostgresRepository) GetForUser(ctx context.Context, userID, orderID uuid.UUID) (models.Delivery, error) {
	var row deliveryRow
	err := r.db.GetContext(ctx, &row, deliverySelect+` JOIN orders o ON o.order_id = d.order_id AND o.created_at = d.order_created_at WHERE d.order_id = $1 AND o.user_id = $2`, orderID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Delivery{}, ErrDeliveryNotFound
	}
	if err != nil {
		return models.Delivery{}, err
	}
	return mapDelivery(row), nil
}

const deliverySelect = `
	SELECT d.delivery_id, d.order_id, d.provider, d.status::text, d.partner_id,
	       u.name AS partner_name, u.phone AS partner_phone,
	       d.assigned_at, d.updated_at, d.next_transition_at
	FROM deliveries d JOIN users u ON u.user_id = d.partner_id`

func mapDelivery(row deliveryRow) models.Delivery {
	return models.Delivery{
		DeliveryID: row.DeliveryID, OrderID: row.OrderID, Provider: row.Provider, Status: row.Status,
		PartnerID: row.PartnerID, PartnerName: row.PartnerName, PartnerPhone: row.PartnerPhone,
		AssignedAt: row.AssignedAt, UpdatedAt: row.UpdatedAt, NextTransitionAt: row.NextTransitionAt,
		DemoLabel: "SIMULATED DELIVERY — no live courier or GPS",
	}
}

func nextStatus(status string) string {
	switch status {
	case models.StatusAssigned:
		return models.StatusEnRouteToRestaurant
	case models.StatusEnRouteToRestaurant:
		return models.StatusArrivedAtRestaurant
	case models.StatusArrivedAtRestaurant:
		return models.StatusPickedUp
	case models.StatusPickedUp:
		return models.StatusOutForDelivery
	case models.StatusOutForDelivery:
		return models.StatusDelivered
	default:
		return ""
	}
}

func orderStatusForDelivery(status string) string {
	switch status {
	case models.StatusEnRouteToRestaurant:
		return "preparing"
	case models.StatusArrivedAtRestaurant:
		return "ready_for_pickup"
	case models.StatusPickedUp:
		return "picked_up"
	case models.StatusOutForDelivery:
		return "out_for_delivery"
	case models.StatusDelivered:
		return "delivered"
	default:
		return "confirmed"
	}
}
