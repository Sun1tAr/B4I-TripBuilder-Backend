package models

import (
	"time"

	"github.com/google/uuid"
)

type Waypoint struct {
	ID              uuid.UUID  `json:"id"`
	EventID         uuid.UUID  `json:"event_id"`
	RouteWaypointID *uuid.UUID `json:"route_waypoint_id,omitempty"`
	City            string     `json:"city"`
	Description     string     `json:"description,omitempty"`
	Latitude        float64    `json:"latitude"`
	Longitude       float64    `json:"longitude"`
	PlannedStart    *time.Time `json:"planned_start,omitempty"`
	PlannedEnd      *time.Time `json:"planned_end,omitempty"`
	ActualStart     *time.Time `json:"actual_start,omitempty"`
	ActualEnd       *time.Time `json:"actual_end,omitempty"`
	Status          string     `json:"status"` // pending, visited, skipped
	OrderIndex      int        `json:"order_index"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}

type WaypointWithMedia struct {
	Waypoint
	Media []Media `json:"media,omitempty"`
}

type VisitWaypointRequest struct {
	ActualStart *time.Time `json:"actual_start,omitempty"`
	ActualEnd   *time.Time `json:"actual_end,omitempty"`
}