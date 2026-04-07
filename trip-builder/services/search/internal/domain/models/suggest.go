package models

// SuggestRequest параметры автодополнения
type SuggestRequest struct {
	Query string `json:"q"`
	Type  string `json:"type"` // routes, events, users
	Size  int    `json:"size"`
}

// SuggestResponse результат автодополнения
type SuggestResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
}

type Suggestion struct {
	Text  string  `json:"text"`
	Score float64 `json:"score"`
	Type  string  `json:"type"` // route, event, user, country, tag
	ID    string  `json:"id,omitempty"`
}
