package models

// UserDocument структура для индекса Elasticsearch
type UserDocument struct {
	ID        string   `json:"id"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Bio       string   `json:"bio,omitempty"`
	Suggest   []string `json:"suggest"` // для completion suggester
}

// UserSearchRequest параметры поиска пользователей
type UserSearchRequest struct {
	Query string `json:"q"`
	Page  int    `json:"page"`
	Size  int    `json:"size"`
}

// UserSearchResponse результат поиска
type UserSearchResponse struct {
	Items      []UserHit `json:"items"`
	Total      int64     `json:"total"`
	Page       int       `json:"page"`
	Size       int       `json:"size"`
	TotalPages int       `json:"total_pages"`
}

type UserHit struct {
	ID         string              `json:"id"`
	FirstName  string              `json:"first_name"`
	LastName   string              `json:"last_name"`
	Bio        string              `json:"bio,omitempty"`
	Score      float64             `json:"_score,omitempty"`
	Highlights map[string][]string `json:"highlights,omitempty"`
}
