package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// JSONMap для работы с JSONB полем data
type JSONMap map[string]interface{}

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, m)
}

// Notification основная модель уведомления
type Notification struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Data      JSONMap   `json:"data,omitempty"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

// NotificationType константы для типов уведомлений
const (
	NotificationTypeEventInvite       = "event_invite"
	NotificationTypeTaskAssigned      = "task_assigned"
	NotificationTypeNewMessage        = "new_message"
	NotificationTypeEventReminder     = "event_reminder"
	NotificationTypeWelcome           = "welcome"
	NotificationTypeEventCancelled    = "event_cancelled"
	NotificationTypeParticipantJoined = "participant_joined"
)

// GetTitleByType возвращает заголовок по умолчанию для типа уведомления
func GetTitleByType(notificationType string) string {
	switch notificationType {
	case NotificationTypeEventInvite:
		return "Приглашение в поездку"
	case NotificationTypeTaskAssigned:
		return "Назначена задача"
	case NotificationTypeNewMessage:
		return "Новое сообщение"
	case NotificationTypeEventReminder:
		return "Напоминание о поездке"
	case NotificationTypeWelcome:
		return "Добро пожаловать!"
	case NotificationTypeEventCancelled:
		return "Поездка отменена"
	case NotificationTypeParticipantJoined:
		return "Новый участник"
	default:
		return "Уведомление"
	}
}
