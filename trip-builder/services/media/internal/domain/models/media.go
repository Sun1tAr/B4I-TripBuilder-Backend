package models

import (
	"time"

	"github.com/google/uuid"
)

type Media struct {
	ID           uuid.UUID  `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	EventID      *uuid.UUID `json:"event_id,omitempty"`
	WaypointID   *uuid.UUID `json:"waypoint_id,omitempty"`
	Purpose      string     `json:"purpose"`
	Filename     string     `json:"filename"`
	ObjectKey    string     `json:"-"`
	ThumbnailKey string     `json:"-"`
	MediumKey    string     `json:"-"`
	MediaType    string     `json:"media_type"`
	FileSize     int64      `json:"file_size"`
	Width        int        `json:"width,omitempty"`
	Height       int        `json:"height,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type MediaResponse struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	MediaType string    `json:"media_type"`
	FileSize  int64     `json:"file_size"`
	Width     int       `json:"width,omitempty"`
	Height    int       `json:"height,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type UploadRequest struct {
	File       []byte     `json:"-"`
	Filename   string     `json:"-"`
	Purpose    string     `json:"purpose" validate:"required,oneof=avatar route_image event_photo waypoint_photo"`
	EventID    *uuid.UUID `json:"event_id,omitempty"`
	WaypointID *uuid.UUID `json:"waypoint_id,omitempty"`
}
