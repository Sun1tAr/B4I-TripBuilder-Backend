package event

import (
	"time"

	"github.com/google/uuid"
)

type CreateEventRequest struct {
	RouteID       uuid.UUID `json:"route_id" validate:"required"`
	Title         string    `json:"title" validate:"required,min=3"`
	Description   string    `json:"description,omitempty"`
	CoverURL      string    `json:"cover_url,omitempty"` // NEW
	StartDate     time.Time `json:"start_date" validate:"required"`
	IsGroup       bool      `json:"is_group"`
	IsPublic      bool      `json:"is_public"`
	InvitedEmails []string  `json:"invited_emails,omitempty"`
}

type UpdateEventRequest struct {
	Title       *string    `json:"title,omitempty"`
	Description *string    `json:"description,omitempty"`
	CoverURL    *string    `json:"cover_url,omitempty"` // NEW
	StartDate   *time.Time `json:"start_date,omitempty"`
	IsPublic    *bool      `json:"is_public,omitempty"`
}

// VisitWaypointRequest DTO для отметки точки посещённой
type VisitWaypointRequest struct {
	ActualStart *time.Time `json:"actual_start,omitempty"`
	ActualEnd   *time.Time `json:"actual_end,omitempty"`
}

// AddMediaRequest DTO для добавления медиа
type AddMediaRequest struct {
	MediaURL     string `json:"media_url" validate:"required,url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	MediumURL    string `json:"medium_url,omitempty"`
	MediaType    string `json:"media_type" validate:"required,oneof=photo video"`
	Description  string `json:"description,omitempty"`
}

// AddCommentRequest DTO для добавления комментария
type AddCommentRequest struct {
	Text string `json:"text" validate:"required"`
}

// UpdateCommentRequest DTO для обновления комментария
type UpdateCommentRequest struct {
	Text string `json:"text" validate:"required"`
}

// InviteRequest DTO для приглашения участников
type InviteRequest struct {
	Emails []string `json:"emails" validate:"required"`
}

// ListEventsRequest DTO для параметров запроса списка поездок
type ListEventsRequest struct {
	Page   int    `json:"page" default:"1"`
	Size   int    `json:"size" default:"10"`
	Status string `json:"status,omitempty"` // planned, active, completed
}

// CreateInviteLinkRequest запрос на создание ссылки-приглашения
type CreateInviteLinkRequest struct {
	EventID    uuid.UUID `json:"event_id" validate:"required"`
	TTLSeconds int       `json:"ttl_seconds,omitempty"` // опционально, если нужен свой TTL
}

// CreateEventTransferRequest DTO для создания перемещения в поездке
type CreateEventTransferRequest struct {
	FromWaypointID  uuid.UUID `json:"from_waypoint_id" validate:"required"`
	ToWaypointID    uuid.UUID `json:"to_waypoint_id" validate:"required"`
	DurationHours   int       `json:"duration_hours" validate:"min=0"`
	DurationMinutes int       `json:"duration_minutes" validate:"min=0,max=59"`
	TransportType   string    `json:"transport_type" validate:"required,oneof=walking car moto animal water air"`
}

// UpdateEventTransferRequest DTO для обновления перемещения
type UpdateEventTransferRequest struct {
	DurationHours   *int       `json:"duration_hours,omitempty"`
	DurationMinutes *int       `json:"duration_minutes,omitempty"`
	TransportType   *string    `json:"transport_type,omitempty"`
	PlannedStart    *time.Time `json:"planned_start,omitempty"`
	PlannedEnd      *time.Time `json:"planned_end,omitempty"`
	ActualStart     *time.Time `json:"actual_start,omitempty"`
	ActualEnd       *time.Time `json:"actual_end,omitempty"`
}

// EventTransferResponse DTO для перемещения в поездке
type EventTransferResponse struct {
	ID              uuid.UUID  `json:"id"`
	FromWaypointID  uuid.UUID  `json:"from_waypoint_id"`
	ToWaypointID    uuid.UUID  `json:"to_waypoint_id"`
	DurationHours   int        `json:"duration_hours"`
	DurationMinutes int        `json:"duration_minutes"`
	TransportType   string     `json:"transport_type"`
	PlannedStart    *time.Time `json:"planned_start,omitempty"`
	PlannedEnd      *time.Time `json:"planned_end,omitempty"`
	ActualStart     *time.Time `json:"actual_start,omitempty"`
	ActualEnd       *time.Time `json:"actual_end,omitempty"`
	OrderIndex      int        `json:"order_index"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}

// TimelineItem элемент таймлайна (точка или перемещение)
type TimelineItem struct {
	Type string      `json:"type"` // "waypoint" или "transfer"
	Data interface{} `json:"data"`
}

// EventDetailResponse DTO для детальной информации о поездке
type EventDetailResponse struct {
	EventResponse
	Participants []ParticipantResponse  `json:"participants"`
	Waypoints    []WaypointResponse     `json:"waypoints"`
	Transfers    []EventTransferResponse `json:"transfers,omitempty"`
	Timeline     []TimelineItem         `json:"timeline,omitempty"` // ← ДОБАВЛЯЕМ
}