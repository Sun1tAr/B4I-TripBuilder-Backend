package event

import (
	"time"

	"github.com/google/uuid"
)

// EventResponse DTO для поездки
type EventResponse struct {
	ID                uuid.UUID  `json:"id"`
	RouteID           uuid.UUID  `json:"route_id"`
	Title             string     `json:"title"`
	Description       string     `json:"description,omitempty"`
	OwnerID           uuid.UUID  `json:"owner_id"`
	StartDate         time.Time  `json:"start_date"`
	EndDate           time.Time  `json:"end_date"`
	IsGroup           bool       `json:"is_group"`
	Status            string     `json:"status"`
	IsPublic          bool       `json:"is_public"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty"`
	TotalInvites      int        `json:"total_invites"`      // все приглашения (!(approved_by_owner = true & approved_by_user = true))
	TotalParticipants int        `json:"total_participants"` // активные участники (approved_by_owner = true, approved_by_user = true)
}

// EventParticipantsStatsDTO статистика участников события
type EventParticipantsStatsDTO struct {
	TotalInvites      int `json:"total_invites"`
	TotalParticipants int `json:"total_participants"`
}

// EventDetailResponse DTO для детальной информации о поездке
type EventDetailResponse struct {
	EventResponse
	Participants []ParticipantResponse `json:"participants"`
	Waypoints    []WaypointResponse    `json:"waypoints"`
}

// ParticipantResponse DTO для участника
type ParticipantResponse struct {
	UserID          uuid.UUID `json:"user_id"`
	Role            string    `json:"role"`
	ApprovedByOwner bool      `json:"approved_by_owner"`
	ApprovedByUser  *bool     `json:"approved_by_user"` // null = приглашение, true/false = принято/отклонено
	InvitedEmail    string    `json:"invited_email,omitempty"`
	JoinedAt        time.Time `json:"joined_at"`
}

// ParticipantWithUserResponse DTO для участника с данными профиля
type ParticipantWithUserResponse struct {
	ParticipantResponse
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// WaypointResponse DTO для точки маршрута
type WaypointResponse struct {
	ID           uuid.UUID  `json:"id"`
	City         string     `json:"city"`
	Description  string     `json:"description,omitempty"`
	Latitude     float64    `json:"latitude"`
	Longitude    float64    `json:"longitude"`
	PlannedStart *time.Time `json:"planned_start,omitempty"`
	PlannedEnd   *time.Time `json:"planned_end,omitempty"`
	ActualStart  *time.Time `json:"actual_start,omitempty"`
	ActualEnd    *time.Time `json:"actual_end,omitempty"`
	Status       string     `json:"status"`
	OrderIndex   int        `json:"order_index"`
}

// WaypointWithMediaResponse DTO для точки с медиа
type WaypointWithMediaResponse struct {
	WaypointResponse
	Media []MediaResponse `json:"media,omitempty"`
}

// MediaResponse DTO для медиа
type MediaResponse struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	MediaURL     string    `json:"media_url"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	MediumURL    string    `json:"medium_url,omitempty"`
	MediaType    string    `json:"media_type"`
	Description  string    `json:"description,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// CommentResponse DTO для комментария
type CommentResponse struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	Text      string     `json:"text"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// ListEventsResponse DTO для списка поездок с пагинацией
type ListEventsResponse struct {
	Items      []EventResponse `json:"items"`
	Total      int64           `json:"total"`
	Page       int             `json:"page"`
	Size       int             `json:"size"`
	TotalPages int             `json:"total_pages"`
}

// InviteResponse DTO для приглашения пользователя
type InviteResponse struct {
	InviteID          uuid.UUID `json:"invite_id"`
	EventID           uuid.UUID `json:"event_id"`
	EventTitle        string    `json:"event_title"`
	EventDescription  string    `json:"event_description,omitempty"`
	EventStartDate    time.Time `json:"event_start_date"`
	EventEndDate      time.Time `json:"event_end_date"`
	EventOwnerID      uuid.UUID `json:"event_owner_id"`
	ParticipantsCount int       `json:"participants_count"`
	UserID            uuid.UUID `json:"user_id"`
	InvitedAt         time.Time `json:"invited_at"`
}

// InviteLinkResponse ответ с ссылкой-приглашением
type InviteLinkResponse struct {
	LinkID    string    `json:"link_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// WaypointPreviewResponse ДТО для точки в предпросмотре поездки (полная версия)
type WaypointPreviewResponse struct {
	ID           uuid.UUID  `json:"id"`
	City         string     `json:"city"`
	Description  string     `json:"description,omitempty"`
	Latitude     float64    `json:"latitude"`
	Longitude    float64    `json:"longitude"`
	PlannedStart *time.Time `json:"planned_start,omitempty"`
	PlannedEnd   *time.Time `json:"planned_end,omitempty"`
	OrderIndex   int        `json:"order_index"`
}

// EventPreviewResponse ДТО для предпросмотра поездки по ссылке-приглашению
type EventPreviewResponse struct {
	ID                uuid.UUID                 `json:"id"`
	Title             string                    `json:"title"`
	Description       string                    `json:"description,omitempty"`
	StartDate         time.Time                 `json:"start_date"`
	EndDate           time.Time                 `json:"end_date"`
	IsGroup           bool                      `json:"is_group"`
	OwnerID           uuid.UUID                 `json:"owner_id"`
	ParticipantsCount int                       `json:"participants_count"`
	Waypoints         []WaypointPreviewResponse `json:"waypoints"`
}
