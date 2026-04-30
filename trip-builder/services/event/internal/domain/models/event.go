package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID          uuid.UUID      `json:"id"`
	RouteID     uuid.UUID      `json:"route_id"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	CoverURL    sql.NullString `json:"cover_url,omitempty"`
	OwnerID     uuid.UUID      `json:"owner_id"`
	StartDate   time.Time      `json:"start_date"`
	EndDate     time.Time      `json:"end_date"`
	IsGroup     bool           `json:"is_group"`
	Status      string         `json:"status"`
	IsPublic    bool           `json:"is_public"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   *time.Time     `json:"updated_at,omitempty"`
}

type EventWithDetails struct {
	Event
	Participants []Participant   `json:"participants"`
	Waypoints    []Waypoint      `json:"waypoints"`
	Transfers    []EventTransfer `json:"transfers,omitempty"` // ← ДОБАВЛЯЕМ
}

type CreateEventRequest struct {
	RouteID       uuid.UUID `json:"route_id" validate:"required"`
	Title         string    `json:"title" validate:"required,min=3"`
	Description   string    `json:"description,omitempty"`
	CoverURL      string    `json:"cover_url,omitempty"`
	StartDate     time.Time `json:"start_date" validate:"required"`
	IsGroup       bool      `json:"is_group"`
	IsPublic      bool      `json:"is_public"`
	InvitedEmails []string  `json:"invited_emails,omitempty"`
}

type UpdateEventRequest struct {
	Title       *string    `json:"title,omitempty"`
	Description *string    `json:"description,omitempty"`
	CoverURL    *string    `json:"cover_url,omitempty"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	IsPublic    *bool      `json:"is_public,omitempty"`
}
