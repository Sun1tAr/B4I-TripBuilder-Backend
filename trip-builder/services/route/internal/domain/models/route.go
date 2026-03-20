package models

import (
	"time"

	"github.com/google/uuid"
)

type Route struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Country     string     `json:"country,omitempty"`
	ImageURL    string     `json:"image_url,omitempty"`
	UserID      uuid.UUID  `json:"user_id"`
	IsPublic    bool       `json:"is_public"`
	TotalDays   int        `json:"total_days"`
	TotalHours  int        `json:"total_hours"`
	LikesCount  int        `json:"likes_count"`
	EventsCount int        `json:"events_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type RouteWithDetails struct {
	Route
	Waypoints []Waypoint `json:"waypoints"`
	Tags      []Tag      `json:"tags"`
	IsLiked   bool       `json:"is_liked,omitempty"` // для текущего пользователя
}

type CreateRouteRequest struct {
	Title       string           `json:"title" validate:"required,min=3"`
	Description string           `json:"description,omitempty"`
	Country     string           `json:"country,omitempty"`
	ImageURL    string           `json:"image_url,omitempty"`
	IsPublic    bool             `json:"is_public"`
	Tags        []string         `json:"tags,omitempty"`
	Waypoints   []CreateWaypoint `json:"waypoints" validate:"required,min=1"`
}

type UpdateRouteRequest struct {
	Title       *string          `json:"title,omitempty"`
	Description *string          `json:"description,omitempty"`
	Country     *string          `json:"country,omitempty"`
	ImageURL    *string          `json:"image_url,omitempty"`
	IsPublic    *bool            `json:"is_public,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	Waypoints   []UpdateWaypoint `json:"waypoints,omitempty"`
}

// RouteWithLike используется для возврата маршрута с информацией о лайке
type RouteWithLike struct {
	Route
	IsLiked bool `json:"is_liked"`
}
