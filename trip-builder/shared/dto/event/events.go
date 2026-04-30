package event

import (
	"time"

	"github.com/google/uuid"
)

// EventCreatedEvent событие создания поездки
type EventCreatedEvent struct {
	EventID   string           `json:"event_id"`
	EventType string           `json:"event_type"`
	Timestamp time.Time        `json:"timestamp"`
	Data      EventCreatedData `json:"data"`
}

type EventCreatedData struct {
	EventID      uuid.UUID   `json:"event_id"`
	RouteID      uuid.UUID   `json:"route_id"`
	OwnerID      uuid.UUID   `json:"owner_id"`
	Title        string      `json:"title"`
	StartDate    time.Time   `json:"start_date"`
	EndDate      time.Time   `json:"end_date"`
	Participants []uuid.UUID `json:"participants"`
	CreatedAt    time.Time   `json:"created_at"`
}

// EventUpdatedEvent событие обновления поездки
type EventUpdatedEvent struct {
	EventID   string           `json:"event_id"`
	EventType string           `json:"event_type"`
	Timestamp time.Time        `json:"timestamp"`
	Data      EventUpdatedData `json:"data"`
}

type EventUpdatedData struct {
	EventID   uuid.UUID `json:"event_id"`
	OwnerID   uuid.UUID `json:"owner_id"`
	Title     string    `json:"title"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	IsPublic  bool      `json:"is_public"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EventCompletedEvent событие завершения поездки
type EventCompletedEvent struct {
	EventID   string             `json:"event_id"`
	EventType string             `json:"event_type"`
	Timestamp time.Time          `json:"timestamp"`
	Data      EventCompletedData `json:"data"`
}

type EventCompletedData struct {
	EventID          uuid.UUID   `json:"event_id"`
	RouteID          uuid.UUID   `json:"route_id"`
	OwnerID          uuid.UUID   `json:"owner_id"`
	Participants     []uuid.UUID `json:"participants"`
	CountriesVisited []string    `json:"countries_visited"`
	CompletedAt      time.Time   `json:"completed_at"`
}

// EventCancelledEvent событие отмены поездки
type EventCancelledEvent struct {
	EventID   string             `json:"event_id"`
	EventType string             `json:"event_type"`
	Timestamp time.Time          `json:"timestamp"`
	Data      EventCancelledData `json:"data"`
}

type EventCancelledData struct {
	EventID     uuid.UUID `json:"event_id"`
	OwnerID     uuid.UUID `json:"owner_id"`
	CancelledAt time.Time `json:"cancelled_at"`
}

// WaypointVisitedEvent событие посещения точки
type WaypointVisitedEvent struct {
	EventID   string              `json:"event_id"`
	EventType string              `json:"event_type"`
	Timestamp time.Time           `json:"timestamp"`
	Data      WaypointVisitedData `json:"data"`
}

type WaypointVisitedData struct {
	EventID    uuid.UUID `json:"event_id"`
	WaypointID uuid.UUID `json:"waypoint_id"`
	UserID     uuid.UUID `json:"user_id"`
	City       string    `json:"city"`
	VisitedAt  time.Time `json:"visited_at"`
}

// ========== EVENT.INVITE ==========

// EventInviteEvent событие приглашения в поездку
type EventInviteEvent struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      EventInviteData `json:"data"`
}

// EventInviteData данные события event.invite
type EventInviteData struct {
	EventID      uuid.UUID     `json:"event_id"`
	EventTitle   string        `json:"event_title"`
	RouteTitle   string        `json:"route_title"`
	InviterID    uuid.UUID     `json:"inviter_id"`
	InviterName  string        `json:"inviter_name"`
	StartDate    time.Time     `json:"start_date"`
	InvitedUsers []InvitedUser `json:"invited_users"`
}

// InvitedUser информация о приглашенном пользователе
type InvitedUser struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
}
