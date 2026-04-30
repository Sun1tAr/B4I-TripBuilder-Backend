package models

import (
	"time"

	"github.com/google/uuid"
)

// TransportType определяет способ перемещения между точками маршрута
type TransportType string

const (
	TransportWalking TransportType = "walking"
	TransportCar     TransportType = "car"
	TransportMoto    TransportType = "moto"
	TransportAnimal  TransportType = "animal"
	TransportWater   TransportType = "water"
	TransportAir     TransportType = "air"
)

// Transfer представляет перемещение между двумя точками маршрута
type Transfer struct {
	ID              uuid.UUID     `json:"id"`
	RouteID         uuid.UUID     `json:"route_id"`
	FromWaypointID  uuid.UUID     `json:"from_waypoint_id"`
	ToWaypointID    uuid.UUID     `json:"to_waypoint_id"`
	DurationHours   int           `json:"duration_hours"`
	DurationMinutes int           `json:"duration_minutes"`
	TransportType   TransportType `json:"transport_type"`
	OrderIndex      int           `json:"order_index"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       *time.Time    `json:"updated_at,omitempty"`
}

// CreateTransferRequest представляет запрос на создание перемещения
type CreateTransferRequest struct {
	FromOrderIndex  int           `json:"from_order_index" validate:"required,min=0"`
	ToOrderIndex    int           `json:"to_order_index" validate:"required,min=0"`
	DurationHours   int           `json:"duration_hours" validate:"min=0"`
	DurationMinutes int           `json:"duration_minutes" validate:"min=0,max=59"`
	TransportType   TransportType `json:"transport_type" validate:"required,oneof=walking car moto animal water air"`
}

// UpdateTransferRequest представляет запрос на обновление перемещения
type UpdateTransferRequest struct {
	ID              *uuid.UUID    `json:"id,omitempty"`
	FromOrderIndex  *int          `json:"from_order_index,omitempty"`
	ToOrderIndex    *int          `json:"to_order_index,omitempty"`
	DurationHours   *int          `json:"duration_hours,omitempty"`
	DurationMinutes *int          `json:"duration_minutes,omitempty"`
	TransportType   *TransportType `json:"transport_type,omitempty"`
}