package chat

import (
	"time"

	"github.com/google/uuid"
)

type MessageResponse struct {
	ID        uuid.UUID              `json:"id"`
	EventID   uuid.UUID              `json:"event_id"`
	UserID    uuid.UUID              `json:"user_id"`
	Text      string                 `json:"text"`
	Type      string                 `json:"type"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt *time.Time             `json:"updated_at,omitempty"`
}

type ListMessagesResponse struct {
	Items      []MessageResponse `json:"items"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	Size       int               `json:"size"`
	TotalPages int               `json:"total_pages"`
}

type UnreadCountResponse struct {
	Count int64 `json:"count"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}
