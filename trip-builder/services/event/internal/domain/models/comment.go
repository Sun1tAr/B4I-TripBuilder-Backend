package models

import (
	"time"

	"github.com/google/uuid"
)

type Comment struct {
	ID         uuid.UUID  `json:"id"`
	EventID    uuid.UUID  `json:"event_id"`
	WaypointID uuid.UUID  `json:"waypoint_id"`
	UserID     uuid.UUID  `json:"user_id"`
	Text       string     `json:"text"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

type AddCommentRequest struct {
	Text string `json:"text" validate:"required"`
}

type UpdateCommentRequest struct {
	Text string `json:"text" validate:"required"`
}
