package notification

import (
	"time"

	"github.com/google/uuid"
)

// NotificationResponse ответ с данными уведомления
type NotificationResponse struct {
	ID        uuid.UUID              `json:"id"`
	Type      string                 `json:"type"`
	Title     string                 `json:"title"`
	Body      string                 `json:"body"`
	Data      map[string]interface{} `json:"data,omitempty"`
	IsRead    bool                   `json:"is_read"`
	CreatedAt time.Time              `json:"created_at"`
}

// ListNotificationsResponse ответ со списком уведомлений
type ListNotificationsResponse struct {
	Items       []NotificationResponse `json:"items"`
	Total       int64                  `json:"total"`
	Page        int                    `json:"page"`
	Size        int                    `json:"size"`
	TotalPages  int                    `json:"total_pages"`
	UnreadCount int64                  `json:"unread_count"`
}

// RegisterDeviceResponse ответ на регистрацию устройства
type RegisterDeviceResponse struct {
	ID       uuid.UUID `json:"id,omitempty"`
	UserID   uuid.UUID `json:"user_id"`
	Token    string    `json:"token"`
	Platform string    `json:"platform"`
	LastUsed time.Time `json:"last_used,omitempty"`
	Message  string    `json:"message"`
}

// MessageResponse простой ответ с сообщением
type MessageResponse struct {
	Message string `json:"message"`
}

// MarkAllAsReadResponse ответ на отметку всех уведомлений
type MarkAllAsReadResponse struct {
	Message string `json:"message"`
	Count   int64  `json:"count"`
}

// UnreadCountResponse ответ с количеством непрочитанных
type UnreadCountResponse struct {
	Count int64 `json:"count"`
}
