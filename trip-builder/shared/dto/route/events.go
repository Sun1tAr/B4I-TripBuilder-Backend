package route

import (
	"time"

	"github.com/google/uuid"
)

// RouteCreatedEvent событие создания маршрута
type RouteCreatedEvent struct {
	EventID   string           `json:"event_id"`
	EventType string           `json:"event_type"`
	Timestamp time.Time        `json:"timestamp"`
	Data      RouteCreatedData `json:"data"`
}

type RouteCreatedData struct {
	RouteID   uuid.UUID `json:"route_id"`
	UserID    uuid.UUID `json:"user_id"`
	Title     string    `json:"title"`
	Country   string    `json:"country,omitempty"`
	IsPublic  bool      `json:"is_public"`
	TotalDays int       `json:"total_days"`
	CreatedAt time.Time `json:"created_at"`
}

// RouteUpdatedEvent событие обновления маршрута
type RouteUpdatedEvent struct {
	EventID   string           `json:"event_id"`
	EventType string           `json:"event_type"`
	Timestamp time.Time        `json:"timestamp"`
	Data      RouteUpdatedData `json:"data"`
}

type RouteUpdatedData struct {
	RouteID   uuid.UUID `json:"route_id"`
	UserID    uuid.UUID `json:"user_id"`
	Title     string    `json:"title"`
	Country   string    `json:"country,omitempty"`
	IsPublic  bool      `json:"is_public"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RouteDeletedEvent событие удаления маршрута
type RouteDeletedEvent struct {
	EventID   string           `json:"event_id"`
	EventType string           `json:"event_type"`
	Timestamp time.Time        `json:"timestamp"`
	Data      RouteDeletedData `json:"data"`
}

type RouteDeletedData struct {
	RouteID uuid.UUID `json:"route_id"`
	UserID  uuid.UUID `json:"user_id"`
}
