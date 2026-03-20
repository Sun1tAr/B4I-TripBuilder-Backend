package models

import (
	"time"

	"github.com/google/uuid"
)

type Like struct {
	UserID    uuid.UUID `json:"user_id"`
	RouteID   uuid.UUID `json:"route_id"`
	CreatedAt time.Time `json:"created_at"`
}
