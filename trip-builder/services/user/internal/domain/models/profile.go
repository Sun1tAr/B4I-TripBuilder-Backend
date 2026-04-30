package models

import (
	"time"

	"github.com/google/uuid"
)

type Profile struct {
	UserID    uuid.UUID `json:"user_id"`
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Bio       string    `json:"bio,omitempty"`
	Phone     string    `json:"phone,omitempty"`
	Language  string    `json:"language"`
	Timezone  string    `json:"timezone"`
	NotificationsEnabled bool      `json:"notifications_enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type UpdateProfileRequest struct {
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Bio       *string `json:"bio,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	Language  *string `json:"language,omitempty"`
	Timezone  *string `json:"timezone,omitempty"`
	NotificationsEnabled *bool `json:"notifications_enabled,omitempty"`
}