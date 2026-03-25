package models

import (
	"time"

	"github.com/google/uuid"
)

type Participant struct {
	InviteID     uuid.UUID `json:"invite_id"`
	EventID      uuid.UUID `json:"event_id"`
	UserID       uuid.UUID `json:"user_id"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	InvitedEmail string    `json:"invited_email,omitempty"`
	JoinedAt     time.Time `json:"joined_at"`
}

type ParticipantWithUser struct {
	Participant
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type InviteRequest struct {
	Emails []string `json:"emails" validate:"required"`
}
