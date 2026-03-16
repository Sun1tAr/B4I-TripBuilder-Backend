package consumer

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// BaseEvent базовая структура всех событий
type BaseEvent struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// UserCreatedData данные события user.created
type UserCreatedData struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
}

// RouteCreatedData данные события route.created
type RouteCreatedData struct {
	RouteID uuid.UUID `json:"route_id"`
	UserID  uuid.UUID `json:"user_id"`
	Country string    `json:"country"`
}

// EventCompletedData данные события event.completed
type EventCompletedData struct {
	EventID          uuid.UUID   `json:"event_id"`
	RouteID          uuid.UUID   `json:"route_id"`
	UserID           uuid.UUID   `json:"owner_id"`
	Participants     []uuid.UUID `json:"participants"`
	CountriesVisited []string    `json:"countries_visited"`
}

// MediaUploadedData данные события media.uploaded
type MediaUploadedData struct {
	MediaID    uuid.UUID  `json:"media_id"`
	EventID    uuid.UUID  `json:"event_id"`
	WaypointID *uuid.UUID `json:"waypoint_id,omitempty"`
	UserID     uuid.UUID  `json:"user_id"`
}