package route

import (
	"github.com/google/uuid"
)

// CreateRouteRequest DTO для создания маршрута
type CreateRouteRequest struct {
	Title       string           `json:"title" validate:"required,min=3"`
	Description string           `json:"description,omitempty"`
	Country     string           `json:"country,omitempty"`
	ImageURL    string           `json:"image_url,omitempty"`
	IsPublic    bool             `json:"is_public"`
	Tags        []string         `json:"tags,omitempty"`
	Waypoints   []CreateWaypoint `json:"waypoints" validate:"required,min=1"`
}

// UpdateRouteRequest DTO для обновления маршрута
type UpdateRouteRequest struct {
	Title       *string          `json:"title,omitempty"`
	Description *string          `json:"description,omitempty"`
	Country     *string          `json:"country,omitempty"`
	ImageURL    *string          `json:"image_url,omitempty"`
	IsPublic    *bool            `json:"is_public,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	Waypoints   []UpdateWaypoint `json:"waypoints,omitempty"`
}

// CreateWaypoint DTO для создания точки маршрута
type CreateWaypoint struct {
	City          string  `json:"city" validate:"required"`
	Description   string  `json:"description,omitempty"`
	Latitude      float64 `json:"latitude" validate:"required,min=-90,max=90"`
	Longitude     float64 `json:"longitude" validate:"required,min=-180,max=180"`
	DurationDays  int     `json:"duration_days" validate:"min=0"`
	DurationHours int     `json:"duration_hours" validate:"min=0,max=23"`
	OrderIndex    int     `json:"order_index"`
}

// UpdateWaypoint DTO для обновления точки маршрута
type UpdateWaypoint struct {
	ID            *uuid.UUID `json:"id,omitempty"`
	City          *string    `json:"city,omitempty"`
	Description   *string    `json:"description,omitempty"`
	Latitude      *float64   `json:"latitude,omitempty"`
	Longitude     *float64   `json:"longitude,omitempty"`
	DurationDays  *int       `json:"duration_days,omitempty"`
	DurationHours *int       `json:"duration_hours,omitempty"`
	OrderIndex    *int       `json:"order_index,omitempty"`
}

// ListRoutesRequest DTO для параметров запроса списка маршрутов
type ListRoutesRequest struct {
	Page         int      `json:"page" default:"1"`
	Size         int      `json:"size" default:"10"`
	Sort         string   `json:"sort" default:"date"` // date, likes
	Country      string   `json:"country,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	DurationFrom *int     `json:"duration_from,omitempty"`
	DurationTo   *int     `json:"duration_to,omitempty"`
}
