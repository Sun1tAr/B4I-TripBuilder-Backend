package models

import "time"

type RouteDocument struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Country     string    `json:"country,omitempty"`
	Tags        []string  `json:"tags"`
	TotalDays   int       `json:"total_days"`
	UserID      string    `json:"user_id"`
	LikesCount  int       `json:"likes_count"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
}

type RouteSearchRequest struct {
	Query        string   `json:"q"`
	Country      string   `json:"country,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	DurationFrom *int     `json:"duration_from,omitempty"`
	DurationTo   *int     `json:"duration_to,omitempty"`
	Sort         string   `json:"sort"`
	Page         int      `json:"page"`
	Size         int      `json:"size"`
	UserID       string   `json:"-"`
}

type RouteSearchResponse struct {
	Items      []RouteHit `json:"items"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	Size       int        `json:"size"`
	TotalPages int        `json:"total_pages"`
}

type RouteHit struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Description string              `json:"description,omitempty"`
	Country     string              `json:"country,omitempty"`
	Tags        []string            `json:"tags"`
	TotalDays   int                 `json:"total_days"`
	UserID      string              `json:"user_id"`
	LikesCount  int                 `json:"likes_count"`
	CreatedAt   time.Time           `json:"created_at"`
	Score       float64             `json:"_score,omitempty"`
	Highlights  map[string][]string `json:"highlights,omitempty"`
}
