package models

import (
	"time"

	"github.com/google/uuid"
)

type UserDevice struct {
	ID       uuid.UUID `json:"id"`
	UserID   uuid.UUID `json:"user_id"`
	Token    string    `json:"token"`
	Platform string    `json:"platform"`
	LastUsed time.Time `json:"last_used"`
}

// Platform константы
const (
	PlatformWeb     = "web"
	PlatformIOS     = "ios"
	PlatformAndroid = "android"
)

type RegisterDeviceRequest struct {
	Token    string `json:"token" validate:"required"`
	Platform string `json:"platform" validate:"required,oneof=web ios android"`
}
