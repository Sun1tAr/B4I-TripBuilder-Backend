package models

import (
	"time"

	"github.com/google/uuid"
)

type Tag struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	RoutesCount int       `json:"routes_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type TagWithRoutes struct {
	Tag
	Routes []Route `json:"routes"`
}
