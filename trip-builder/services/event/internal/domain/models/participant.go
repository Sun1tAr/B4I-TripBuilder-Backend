package models

import (
	"time"

	"github.com/google/uuid"
)

type Participant struct {
	InviteID        uuid.UUID `json:"invite_id"`
	EventID         uuid.UUID `json:"event_id"`
	UserID          uuid.UUID `json:"user_id"`
	Role            string    `json:"role"` // owner, member
	ApprovedByOwner bool      `json:"approved_by_owner"`
	ApprovedByUser  bool      `json:"approved_by_user"`
	InvitedEmail    string    `json:"invited_email,omitempty"`
	JoinedAt        time.Time `json:"joined_at"`
}

// IsActive возвращает true, если участник полностью подтверждён
func (p *Participant) IsActive() bool {
	return p.ApprovedByOwner && p.ApprovedByUser
}

type Invite struct {
	InviteID          uuid.UUID `json:"invite_id"`
	EventID           uuid.UUID `json:"event_id"`
	EventTitle        string    `json:"event_title"`
	EventDescription  string    `json:"event_description,omitempty"`
	EventStartDate    time.Time `json:"event_start_date"`
	EventEndDate      time.Time `json:"event_end_date"`
	EventOwnerID      uuid.UUID `json:"event_owner_id"`
	ParticipantsCount int       `json:"participants_count"`
	InvitedBy         uuid.UUID `json:"invited_by"`
	InvitedAt         time.Time `json:"invited_at"`
}
