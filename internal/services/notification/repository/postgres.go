package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"food-delivery-backend/internal/services/notification/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type PostgresRepository struct{ db *sqlx.DB }

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository { return &PostgresRepository{db: db} }

type notificationRow struct {
	NotificationID uuid.UUID `db:"notification_id"`
	Title          string    `db:"title"`
	Body           string    `db:"body"`
	Channels       []string  `db:"channels"`
	Status         string    `db:"status"`
	IsRead         bool      `db:"is_read"`
	CreatedAt      time.Time `db:"created_at"`
}

func (r *PostgresRepository) List(ctx context.Context, recipientID uuid.UUID, unreadOnly bool, limit int) (models.ListResult, error) {
	rows := make([]notificationRow, 0)
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT notification_id, title, body, channels, status, is_read, created_at
		FROM notifications
		WHERE recipient_id = $1 AND ($2 = FALSE OR is_read = FALSE)
		ORDER BY created_at DESC
		LIMIT $3`, recipientID, unreadOnly, limit); err != nil {
		return models.ListResult{}, err
	}
	var unread int
	if err := r.db.GetContext(ctx, &unread, `SELECT count(*) FROM notifications WHERE recipient_id = $1 AND is_read = FALSE`, recipientID); err != nil {
		return models.ListResult{}, err
	}
	items := make([]models.Notification, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapNotification(row))
	}
	return models.ListResult{Items: items, UnreadCount: unread}, nil
}

func (r *PostgresRepository) MarkRead(ctx context.Context, recipientID, notificationID uuid.UUID) (models.Notification, error) {
	var row notificationRow
	err := r.db.GetContext(ctx, &row, `
		UPDATE notifications
		SET is_read = TRUE, updated_at = NOW()
		WHERE notification_id = $1 AND recipient_id = $2
		RETURNING notification_id, title, body, channels, status, is_read, created_at`, notificationID, recipientID)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Notification{}, ErrNotificationNotFound
	}
	if err != nil {
		return models.Notification{}, err
	}
	return mapNotification(row), nil
}

func (r *PostgresRepository) MarkAllRead(ctx context.Context, recipientID uuid.UUID) (int, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE notifications SET is_read = TRUE, updated_at = NOW()
		WHERE recipient_id = $1 AND is_read = FALSE`, recipientID)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	return int(count), err
}

func mapNotification(row notificationRow) models.Notification {
	return models.Notification{NotificationID: row.NotificationID, Title: row.Title, Body: row.Body, Channels: row.Channels, Status: row.Status, IsRead: row.IsRead, CreatedAt: row.CreatedAt}
}
