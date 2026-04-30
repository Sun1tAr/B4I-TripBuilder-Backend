package models

import (
	"github.com/google/uuid"
)

// RouteWaypoint — структура, получаемая от Route Service
type RouteWaypoint struct {
	ID            uuid.UUID `json:"id"`
	City          string    `json:"city"`
	Description   string    `json:"description,omitempty"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	DurationDays  int       `json:"duration_days"`
	DurationHours int       `json:"duration_hours"`
	OrderIndex    int       `json:"order_index"`
}

// RouteWithWaypoints — полный ответ от Route Service
type RouteWithWaypoints struct {
	ID          uuid.UUID       `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Country     string          `json:"country,omitempty"`
	ImageURL    string          `json:"image_url,omitempty"`
	UserID      uuid.UUID       `json:"user_id"`
	IsPublic    bool            `json:"is_public"`
	Waypoints   []RouteWaypoint `json:"waypoints"`
}
