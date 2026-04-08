package models

type UserDocument struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Bio       string `json:"bio,omitempty"`
}

type UserSearchRequest struct {
	Query string `json:"q"`
	Page  int    `json:"page"`
	Size  int    `json:"size"`
}

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
