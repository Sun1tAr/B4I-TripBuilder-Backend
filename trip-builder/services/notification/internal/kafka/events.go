package kafka

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// BaseEvent базовая структура всех событий Kafka
type BaseEvent struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// UserWelcomeData данные события user.welcome
type UserWelcomeData struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
}

// EventInviteData данные события event.invite
type EventInviteData struct {
	EventID      uuid.UUID     `json:"event_id"`
	EventTitle   string        `json:"event_title"`
	RouteTitle   string        `json:"route_title"`
	InviterID    uuid.UUID     `json:"inviter_id"`
	InviterName  string        `json:"inviter_name"`
	StartDate    time.Time     `json:"start_date"`
	InvitedUsers []InvitedUser `json:"invited_users"`
}

// InvitedUser информация о приглашенном пользователе
type InvitedUser struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
}

// TaskAssignedData данные события task.assigned
type TaskAssignedData struct {
	TaskID          uuid.UUID `json:"task_id"`
	TaskTitle       string    `json:"task_title"`
	TaskDescription string    `json:"task_description"`
	EventID         uuid.UUID `json:"event_id"`
	EventTitle      string    `json:"event_title"`
	AssignedTo      uuid.UUID `json:"assigned_to"`
	AssignedEmail   string    `json:"assigned_email"`
	AssignedName    string    `json:"assigned_name"`
	Priority        string    `json:"priority"`
	DueDate         time.Time `json:"due_date"`
	AssignedBy      uuid.UUID `json:"assigned_by"`
	AssignedByName  string    `json:"assigned_by_name"`
}

// ChatMessageData данные события chat.message
type ChatMessageData struct {
	MessageID   uuid.UUID `json:"message_id"`
	EventID     uuid.UUID `json:"event_id"`
	EventTitle  string    `json:"event_title"`
	SenderID    uuid.UUID `json:"sender_id"`
	SenderName  string    `json:"sender_name"`
	MessageText string    `json:"message_text"`
	SentAt      time.Time `json:"sent_at"`
	// OfflineRecipients список пользователей, которые офлайн и должны получить уведомление
	OfflineRecipients []OfflineRecipient `json:"offline_recipients,omitempty"`
}

// OfflineRecipient информация об офлайн-получателе
type OfflineRecipient struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
}

// EventReminderData данные события event.reminder
type EventReminderData struct {
	EventID      uuid.UUID             `json:"event_id"`
	EventTitle   string                `json:"event_title"`
	StartDate    time.Time             `json:"start_date"`
	DaysLeft     int                   `json:"days_left"`
	Participants []ReminderParticipant `json:"participants"`
}

// ReminderParticipant участник для напоминания
type ReminderParticipant struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
}
