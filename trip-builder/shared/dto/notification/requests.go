package notification

import (
	"time"

	"github.com/google/uuid"
)

// RegisterDeviceRequest запрос на регистрацию устройства
type RegisterDeviceRequest struct {
	Token    string `json:"token" validate:"required"`
	Platform string `json:"platform" validate:"required,oneof=web ios android"`
}

// MarkAsReadRequest запрос на отметку прочитанным
type MarkAsReadRequest struct {
	NotificationID uuid.UUID `json:"notification_id" validate:"required"`
}

// ListNotificationsRequest параметры запроса списка уведомлений
type ListNotificationsRequest struct {
	Page     int        `json:"page" default:"1"`
	Size     int        `json:"size" default:"20"`
	IsRead   *bool      `json:"is_read,omitempty"`
	Type     string     `json:"type,omitempty"`
	DateFrom *time.Time `json:"date_from,omitempty"`
	DateTo   *time.Time `json:"date_to,omitempty"`
}
