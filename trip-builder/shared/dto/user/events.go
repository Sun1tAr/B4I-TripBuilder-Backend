package user

import (
	"time"

	"github.com/google/uuid"
)

type AchievementEarnedEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	Data      AchievementEarnedData `json:"data"`
}

type AchievementEarnedData struct {
	UserID          uuid.UUID `json:"user_id"`
	AchievementType string    `json:"achievement_type"`
	Title           string    `json:"title"`
}