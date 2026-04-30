package models

import (
	"time"

	"github.com/google/uuid"
)

// TransportType определяет способ перемещения между точками
type TransportType string

const (
	TransportWalking TransportType = "walking"
	TransportCar     TransportType = "car"
	TransportMoto    TransportType = "moto"
	TransportAnimal  TransportType = "animal"
	TransportWater   TransportType = "water"
	TransportAir     TransportType = "air"
)

// EventTransfer представляет перемещение между точками в поездке
type EventTransfer struct {
	ID              uuid.UUID     `json:"id"`
	EventID         uuid.UUID     `json:"event_id"`
	FromWaypointID  uuid.UUID     `json:"from_waypoint_id"`
	ToWaypointID    uuid.UUID     `json:"to_waypoint_id"`
	DurationHours   int           `json:"duration_hours"`
	DurationMinutes int           `json:"duration_minutes"`
	TransportType   TransportType `json:"transport_type"`
	PlannedStart    *time.Time    `json:"planned_start,omitempty"`
	PlannedEnd      *time.Time    `json:"planned_end,omitempty"`
	ActualStart     *time.Time    `json:"actual_start,omitempty"`
	ActualEnd       *time.Time    `json:"actual_end,omitempty"`
	OrderIndex      int           `json:"order_index"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       *time.Time    `json:"updated_at,omitempty"`
}

// CreateEventTransferRequest запрос на создание перемещения в поездке
type CreateEventTransferRequest struct {
	FromWaypointID  uuid.UUID     `json:"from_waypoint_id" validate:"required"`
	ToWaypointID    uuid.UUID     `json:"to_waypoint_id" validate:"required"`
	DurationHours   int           `json:"duration_hours" validate:"min=0"`
	DurationMinutes int           `json:"duration_minutes" validate:"min=0,max=59"`
	TransportType   TransportType `json:"transport_type" validate:"required,oneof=walking car moto animal water air"`
}

// UpdateEventTransferRequest запрос на обновление перемещения
type UpdateEventTransferRequest struct {
	DurationHours   *int           `json:"duration_hours,omitempty"`
	DurationMinutes *int           `json:"duration_minutes,omitempty"`
	TransportType   *TransportType `json:"transport_type,omitempty"`
	PlannedStart    *time.Time     `json:"planned_start,omitempty"`
	PlannedEnd      *time.Time     `json:"planned_end,omitempty"`
	ActualStart     *time.Time     `json:"actual_start,omitempty"`
	ActualEnd       *time.Time     `json:"actual_end,omitempty"`
}