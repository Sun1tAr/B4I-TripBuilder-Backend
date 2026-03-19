package models

import (
	"time"

	"github.com/google/uuid"
)

type Waypoint struct {
	ID            uuid.UUID  `json:"id"`
	RouteID       uuid.UUID  `json:"route_id"`
	City          string     `json:"city"`
	Description   string     `json:"description,omitempty"`
	Latitude      float64    `json:"latitude"`
	Longitude     float64    `json:"longitude"`
	DurationDays  int        `json:"duration_days"`
	DurationHours int        `json:"duration_hours"`
	OrderIndex    int        `json:"order_index"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

type CreateWaypoint struct {
	City          string  `json:"city" validate:"required"`
	Description   string  `json:"description,omitempty"`
	Latitude      float64 `json:"latitude" validate:"required,min=-90,max=90"`
	Longitude     float64 `json:"longitude" validate:"required,min=-180,max=180"`
	DurationDays  int     `json:"duration_days" validate:"min=0"`
	DurationHours int     `json:"duration_hours" validate:"min=0,max=23"`
	OrderIndex    int     `json:"order_index"`
}

type UpdateWaypoint struct {
	ID            *uuid.UUID `json:"id,omitempty"` // для существующих точек
	City          *string    `json:"city,omitempty"`
	Description   *string    `json:"description,omitempty"`
	Latitude      *float64   `json:"latitude,omitempty"`
	Longitude     *float64   `json:"longitude,omitempty"`
	DurationDays  *int       `json:"duration_days,omitempty"`
	DurationHours *int       `json:"duration_hours,omitempty"`
	OrderIndex    *int       `json:"order_index,omitempty"`
	// Если ID не указан, точка будет создана как новая
}
