package models

import (
	"time"

	"github.com/google/uuid"
)

type Media struct {
	ID           uuid.UUID `json:"id"`
	EventID      uuid.UUID `json:"event_id"`
	WaypointID   uuid.UUID `json:"waypoint_id"`
	UserID       uuid.UUID `json:"user_id"`
	MediaURL     string    `json:"media_url"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	MediumURL    string    `json:"medium_url,omitempty"`
	MediaType    string    `json:"media_type"` // photo, video
	Description  string    `json:"description,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type AddMediaRequest struct {
	MediaURL     string `json:"media_url" validate:"required,url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	MediumURL    string `json:"medium_url,omitempty"`
	MediaType    string `json:"media_type" validate:"required,oneof=photo video"`
	Description  string `json:"description,omitempty"`
}
