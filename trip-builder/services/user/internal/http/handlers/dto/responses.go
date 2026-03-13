package dto

import (
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/models"
	"github.com/google/uuid"
)

type ProfileResponse struct {
	UserID    uuid.UUID `json:"user_id"`
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Bio       string    `json:"bio,omitempty"`
	Language  string    `json:"language"`
	Timezone  string    `json:"timezone"`
	NotificationsEnabled bool      `json:"notifications_enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type PublicProfileResponse struct {
	UserID    uuid.UUID `json:"user_id"`
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Bio       string    `json:"bio,omitempty"`
}

type StatsResponse struct {
	UserID         uuid.UUID `json:"user_id"`
	RoutesCreated  int       `json:"routes_created"`
	EventsDone     int       `json:"events_done"`
	CountriesVisited int     `json:"countries_visited"`
	PhotosUploaded int       `json:"photos_uploaded"`
	Followers      int       `json:"followers"`
	Following      int       `json:"following"`
}

type AchievementResponse struct {
	ID          uuid.UUID `json:"id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	EarnedAt    time.Time `json:"earned_at"`
}

type FollowResponse struct {
	Followers []uuid.UUID `json:"followers"`
	Following []uuid.UUID `json:"following"`
	Total     int         `json:"total"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

func ProfileToResponse(p *models.Profile) *ProfileResponse {
	return &ProfileResponse{
		UserID:    p.UserID,
		FirstName: p.FirstName,
		LastName:  p.LastName,
		AvatarURL: p.AvatarURL,
		Bio:       p.Bio,
		Language:  p.Language,
		Timezone:  p.Timezone,
		NotificationsEnabled: p.NotificationsEnabled,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func ProfileToPublicResponse(p *models.Profile) *PublicProfileResponse {
	return &PublicProfileResponse{
		UserID:    p.UserID,
		FirstName: p.FirstName,
		LastName:  p.LastName,
		AvatarURL: p.AvatarURL,
		Bio:       p.Bio,
	}
}



func AchievementToResponse(a *models.Achievement) *AchievementResponse {
	return &AchievementResponse{
		ID:          a.ID,
		Type:        a.Type,
		Title:       a.Title,
		Description: a.Description,
		EarnedAt:    a.EarnedAt,
	}
}