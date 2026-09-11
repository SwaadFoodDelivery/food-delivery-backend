package models

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	NotificationID uuid.UUID `json:"notification_id"`
	Title          string    `json:"title"`
	Body           string    `json:"body"`
	Channels       []string  `json:"channels"`
	Status         string    `json:"status"`
	IsRead         bool      `json:"is_read"`
	CreatedAt      time.Time `json:"created_at"`
}

type ListResult struct {
	Items       []Notification `json:"items"`
	UnreadCount int            `json:"unread_count"`
}
