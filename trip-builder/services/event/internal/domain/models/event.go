package models

import (
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID          uuid.UUID  `json:"id"`
	RouteID     uuid.UUID  `json:"route_id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	OwnerID     uuid.UUID  `json:"owner_id"`
	StartDate   time.Time  `json:"start_date"`
	EndDate     time.Time  `json:"end_date"`
	IsGroup     bool       `json:"is_group"`
	Status      string     `json:"status"` // planned, active, completed, cancelled
	IsPublic    bool       `json:"is_public"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type EventWithDetails struct {
	Event
	Participants []Participant `json:"participants"`
	Waypoints    []Waypoint    `json:"waypoints"`
}

type CreateEventRequest struct {
	RouteID       uuid.UUID `json:"route_id" validate:"required"`
	Title         string    `json:"title" validate:"required,min=3"`
	Description   string    `json:"description,omitempty"`
	StartDate     time.Time `json:"start_date" validate:"required"`
	IsGroup       bool      `json:"is_group"`
	InvitedEmails []string  `json:"invited_emails,omitempty"`
}

type UpdateEventRequest struct {
	Title       *string    `json:"title,omitempty"`
	Description *string    `json:"description,omitempty"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	IsPublic    *bool      `json:"is_public,omitempty"`
}
