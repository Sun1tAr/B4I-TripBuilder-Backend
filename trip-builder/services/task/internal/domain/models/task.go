package models

import (
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID          uuid.UUID  `json:"id"`
	EventID     uuid.UUID  `json:"event_id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	AssignedTo  *uuid.UUID `json:"assigned_to,omitempty"` // nil = всем
	CreatedBy   uuid.UUID  `json:"created_by"`
	Priority    string     `json:"priority"` // high, medium, low
	Status      string     `json:"status"`   // pending, done, cancelled
	DueDate     *time.Time `json:"due_date,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type TaskComment struct {
	ID        uuid.UUID `json:"id"`
	TaskID    uuid.UUID `json:"task_id"`
	UserID    uuid.UUID `json:"user_id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskTemplate struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	TriggerDays int       `json:"trigger_days"` // отрицательное = до старта
	Priority    string    `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
}

// Request/Response DTO
type CreateTaskRequest struct {
	EventID     uuid.UUID  `json:"event_id" validate:"required"`
	Title       string     `json:"title" validate:"required,min=3"`
	Description string     `json:"description,omitempty"`
	AssignedTo  *uuid.UUID `json:"assigned_to,omitempty"`
	Priority    string     `json:"priority" validate:"oneof=high medium low"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}

type UpdateTaskRequest struct {
	Title       *string    `json:"title,omitempty"`
	Description *string    `json:"description,omitempty"`
	AssignedTo  *uuid.UUID `json:"assigned_to,omitempty"`
	Priority    *string    `json:"priority,omitempty"`
	Status      *string    `json:"status,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}

type AddCommentRequest struct {
	Text string `json:"text" validate:"required"`
}

type ListTasksRequest struct {
	EventID    *uuid.UUID `json:"event_id,omitempty"`
	AssignedTo *uuid.UUID `json:"assigned_to,omitempty"`
	Status     string     `json:"status,omitempty"`
	Page       int        `json:"page"`
	Size       int        `json:"size"`
}
