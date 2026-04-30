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
	Transfers []Transfer `json:"transfers,omitempty"`
	IsLiked   bool       `json:"is_liked,omitempty"`
}

type CreateRouteRequest struct {
	Title       string                 `json:"title" validate:"required,min=3"`
	Description string                 `json:"description,omitempty"`
	Country     string                 `json:"country,omitempty"`
	ImageURL    string                 `json:"image_url,omitempty"`
	IsPublic    bool                   `json:"is_public"`
	Tags        []string               `json:"tags,omitempty"`
	Waypoints   []CreateWaypoint       `json:"waypoints" validate:"required,min=1"`
	Transfers   []CreateTransferRequest `json:"transfers,omitempty"`
}

type UpdateRouteRequest struct {
	Title       *string                `json:"title,omitempty"`
	Description *string                `json:"description,omitempty"`
	Country     *string                `json:"country,omitempty"`
	ImageURL    *string                `json:"image_url,omitempty"`
	IsPublic    *bool                  `json:"is_public,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Waypoints   []UpdateWaypoint       `json:"waypoints,omitempty"`
	Transfers   []UpdateTransferRequest `json:"transfers,omitempty"`
}

type RouteWithLike struct {
	Route
	IsLiked bool `json:"is_liked"`
}