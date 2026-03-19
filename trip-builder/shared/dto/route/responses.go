package route

import (
	"time"

	"github.com/google/uuid"
)

// RouteResponse DTO для маршрута
type RouteResponse struct {
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
	IsLiked     bool       `json:"is_liked,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// RouteDetailResponse DTO для детальной информации о маршруте
type RouteDetailResponse struct {
	RouteResponse
	Waypoints []WaypointResponse `json:"waypoints"`
	Tags      []TagResponse      `json:"tags"`
}

// WaypointResponse DTO для точки маршрута
type WaypointResponse struct {
	ID            uuid.UUID `json:"id"`
	City          string    `json:"city"`
	Description   string    `json:"description,omitempty"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	DurationDays  int       `json:"duration_days"`
	DurationHours int       `json:"duration_hours"`
	OrderIndex    int       `json:"order_index"`
}

// TagResponse DTO для тега
type TagResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RoutesCount int    `json:"routes_count"`
}

// LikeResponse DTO для лайка
type LikeResponse struct {
	UserID    uuid.UUID `json:"user_id"`
	RouteID   uuid.UUID `json:"route_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ListRoutesResponse DTO для списка маршрутов с пагинацией
type ListRoutesResponse struct {
	Items      []RouteResponse `json:"items"`
	Total      int64           `json:"total"`
	Page       int             `json:"page"`
	Size       int             `json:"size"`
	TotalPages int             `json:"total_pages"`
}
