package models

import (
	"github.com/google/uuid"
)

type Stats struct {
	UserID         uuid.UUID `json:"user_id"`
	RoutesCreated  int       `json:"routes_created"`
	EventsDone     int       `json:"events_done"`
	CountriesVisited int     `json:"countries_visited"`
	PhotosUploaded int       `json:"photos_uploaded"`
	Followers      int       `json:"followers"`
	Following      int       `json:"following"`
}