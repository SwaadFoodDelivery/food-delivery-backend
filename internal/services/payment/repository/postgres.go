package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"food-delivery-backend/internal/services/payment/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type PostgresRepository struct{ db *sqlx.DB }

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository { return &PostgresRepository{db: db} }

type orderRow struct {
	OrderID       uuid.UUID `db:"order_id"`
	CreatedAt     time.Time `db:"created_at"`
	UserID        uuid.UUID `db:"user_id"`
	TotalAmount   string    `db:"total_amount"`
	PaymentMethod string    `db:"payment_method"`
	Status        string    `db:"status"`
}

type paymentRow struct {
	PaymentID         uuid.UUID      `db:"payment_id"`
	OrderID           uuid.UUID      `db:"order_id"`
	Provider          string         `db:"provider"`
	Status            string         `db:"status"`
	ProviderPaymentID sql.NullString `db:"provider_payment_id"`
	FailureCode       sql.NullString `db:"failure_code"`
	FailureMessage    sql.NullString `db:"failure_message"`
	Amount            string         `db:"amount"`
	CreatedAt         time.Time      `db:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at"`
}

func (r *PostgresRepository) FindOrderForPayment(ctx context.Context, userID, orderID uuid.UUID) (models.Order, error) {
	var row orderRow
	err := r.db.GetContext(ctx, &row, `
		SELECT order_id, created_at, user_id, total_amount::text, payment_method, status::text
		FROM orders
		WHERE order_id = $1 AND user_id = $2
		ORDER BY created_at DESC LIMIT 1`, orderID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Order{}, ErrOrderNotFound
	}
	if err != nil {
		return models.Order{}, err
	}
	return models.Order{
		OrderID: row.OrderID, CreatedAt: row.CreatedAt, UserID: row.UserID,
		TotalAmount: mustMinor(row.TotalAmount), PaymentMethod: row.PaymentMethod, Status: row.Status,
	}, nil
}

func (r *PostgresRepository) FindByIdempotency(ctx context.Context, key string) (models.Payment, bool, error) {
	var row paymentRow
	err := r.db.GetContext(ctx, &row, paymentSelect+` WHERE p.idempotency_key = $1`, strings.TrimSpace(key))
	if errors.Is(err, sql.ErrNoRows) {
		return models.Payment{}, false, nil
	}
	if err != nil {
		return models.Payment{}, false, err
	}
	return mapPayment(row), true, nil
}

func (r *PostgresRepository) CreatePending(ctx context.Context, payment models.Payment, key string) (models.Payment, bool, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return models.Payment{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	// Different retry keys still represent the same payable order. Lock that
	// order before checking prior attempts so concurrent requests cannot charge
	// it twice, and a lost successful response is safely replayed.
	var status string
	if err := tx.GetContext(ctx, &status, `SELECT status::text FROM orders
		WHERE order_id=$1 AND created_at=$2 AND user_id=$3 FOR UPDATE`, payment.OrderID, payment.CreatedAt, payment.UserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Payment{}, false, ErrOrderNotFound
		}
		return models.Payment{}, false, err
	}
	if status == "cancelled" || status == "rejected" || status == "delivered" {
		return models.Payment{}, false, ErrOrderNotPayable
	}
	var previous paymentRow
	err = tx.GetContext(ctx, &previous, paymentSelect+` WHERE p.order_id=$1 AND p.order_created_at=$2
		AND p.status IN ('pending','success') ORDER BY p.created_at DESC LIMIT 1`, payment.OrderID, payment.CreatedAt)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return models.Payment{}, false, err
		}
		return mapPayment(previous), true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return models.Payment{}, false, err
	}

	var paymentID uuid.UUID
	err = tx.GetContext(ctx, &paymentID, `
		INSERT INTO payments (order_id, order_created_at, user_id, amount, status, idempotency_key, provider)
		VALUES ($1, $2, $3, $4::decimal, 'pending', $5, $6)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING payment_id`, payment.OrderID, payment.CreatedAt, payment.UserID, minorDecimal(payment.Amount), strings.TrimSpace(key), payment.Provider)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return models.Payment{}, false, err
		}
		existing, found, err := r.FindByIdempotency(ctx, key)
		return existing, found, err
	}
	if err != nil {
		return models.Payment{}, false, err
	}
	var row paymentRow
	if err := tx.GetContext(ctx, &row, paymentSelect+` WHERE p.payment_id = $1`, paymentID); err != nil {
		return models.Payment{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return models.Payment{}, false, err
	}
	return mapPayment(row), false, nil
}

func (r *PostgresRepository) Complete(ctx context.Context, paymentID uuid.UUID, status, providerPaymentID, failureCode, failureMessage string) (models.Payment, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return models.Payment{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var row paymentRow
	err = tx.GetContext(ctx, &row, paymentSelect+` WHERE p.payment_id = $1 FOR UPDATE`, paymentID)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Payment{}, ErrPaymentNotFound
	}
	if err != nil {
		return models.Payment{}, err
	}
	if row.Status != models.StatusPending {
		if err := tx.Commit(); err != nil {
			return models.Payment{}, err
		}
		return mapPayment(row), nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE payments
		SET status = $1, provider_payment_id = NULLIF($2, ''), failure_code = NULLIF($3, ''), failure_message = NULLIF($4, ''), updated_at = NOW()
		WHERE payment_id = $5 AND status = 'pending'`, status, providerPaymentID, failureCode, failureMessage, paymentID); err != nil {
		return models.Payment{}, err
	}
	if err := tx.GetContext(ctx, &row, paymentSelect+` WHERE p.payment_id = $1`, paymentID); err != nil {
		return models.Payment{}, err
	}
	title, body := "Payment confirmed", "Your mock payment was confirmed for the demo order."
	if status == models.StatusFailed {
		title, body = "Payment needs attention", "Your mock payment was declined. You can retry the demo payment."
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO notifications (recipient_id, recipient_type, channels, title, body, status)
		SELECT o.user_id, 'client', ARRAY['in_app'], $1, $2, 'queued'
		FROM orders o WHERE o.order_id = $3`, title, body, row.OrderID); err != nil {
		return models.Payment{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.Payment{}, err
	}
	return mapPayment(row), nil
}

const paymentSelect = `
	SELECT p.payment_id, p.order_id, p.provider, p.status, p.provider_payment_id,
	       p.failure_code, p.failure_message, p.amount::text, p.created_at, p.updated_at
	FROM payments p`

func mapPayment(row paymentRow) models.Payment {
	return models.Payment{
		PaymentID: row.PaymentID, OrderID: row.OrderID, Provider: row.Provider, Status: row.Status,
		ProviderPaymentID: row.ProviderPaymentID.String, FailureCode: row.FailureCode.String,
		FailureMessage: row.FailureMessage.String, Amount: mustMinor(row.Amount), Currency: "INR",
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func minorDecimal(minor int64) string { return fmt.Sprintf("%d.%02d", minor/100, minor%100) }

func mustMinor(raw string) int64 {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) > 2 {
		return 0
	}
	major, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || major < 0 {
		return 0
	}
	fractionText := ""
	if len(parts) == 2 {
		fractionText = parts[1]
	}
	if len(fractionText) > 2 {
		return 0
	}
	for len(fractionText) < 2 {
		fractionText += "0"
	}
	fraction, err := strconv.ParseInt(fractionText, 10, 64)
	if err != nil || fraction < 0 {
		return 0
	}
	return major*100 + fraction
}
