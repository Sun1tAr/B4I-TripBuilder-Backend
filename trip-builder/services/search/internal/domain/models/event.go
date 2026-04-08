package models

import "time"

type EventDocument struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	RouteID        string    `json:"route_id"`
	StartDate      time.Time `json:"start_date"`
	Status         string    `json:"status"`
	ParticipantIDs []string  `json:"participant_ids"`
	IsPublic       bool      `json:"is_public"`
}

type EventSearchRequest struct {
	Query  string `json:"q"`
	Status string `json:"status,omitempty"`
	Page   int    `json:"page"`
	Size   int    `json:"size"`
}

type EventSearchResponse struct {
	Items      []EventHit `json:"items"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	Size       int        `json:"size"`
	TotalPages int        `json:"total_pages"`
}

type EventHit struct {
	ID         string              `json:"id"`
	Title      string              `json:"title"`
	RouteID    string              `json:"route_id"`
	StartDate  time.Time           `json:"start_date"`
	Status     string              `json:"status"`
	Score      float64             `json:"_score,omitempty"`
	Highlights map[string][]string `json:"highlights,omitempty"`
}
