package models

import (
	"time"

	"github.com/google/uuid"
)

type EmailTemplate struct {
	ID           uuid.UUID  `json:"id"`
	Type         string     `json:"type"`
	Subject      string     `json:"subject"`
	BodyTemplate string     `json:"body_template"`
	Language     string     `json:"language"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
}

// TemplateType константы
const (
	TemplateTypeWelcome       = "user.welcome"
	TemplateTypeEventInvite   = "event.invite"
	TemplateTypeTaskAssigned  = "task.assigned"
	TemplateTypeEventReminder = "event.reminder"
	TemplateTypeChatMessage   = "chat.message"
)
