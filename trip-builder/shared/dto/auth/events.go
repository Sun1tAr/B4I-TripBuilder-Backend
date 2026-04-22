package auth

import (
	"time"

	"github.com/google/uuid"
)

type UserCreatedEvent struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      UserCreatedData `json:"data"`
}

type UserCreatedData struct {
	UserID            uuid.UUID `json:"user_id"`
	Email             string    `json:"email"`
	FirstName         string    `json:"first_name"`
	LastName          string    `json:"last_name"`
	VerificationToken string    `json:"verification_token"`
}
