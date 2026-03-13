package models

import (
	"time"

	"github.com/google/uuid"
)

type Achievement struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	EarnedAt    time.Time `json:"earned_at"`
}

type AchievementType string

const (
	AchievementFirstRoute   AchievementType = "first_route"
	AchievementFirstEvent   AchievementType = "first_event"
	AchievementFirstCountry AchievementType = "first_country"
	AchievementFirstPhoto   AchievementType = "first_photo"
	AchievementFirstFollower AchievementType = "first_follower"
	AchievementFirstFollowing AchievementType = "first_following"
	AchievementTenRoutes    AchievementType = "ten_routes"
	AchievementTenEvents    AchievementType = "ten_events"
	AchievementTenCountries AchievementType = "ten_countries"
	AchievementHundredPhotos AchievementType = "hundred_photos"
)