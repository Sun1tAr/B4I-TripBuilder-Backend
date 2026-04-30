package task

import (
	"time"

	"github.com/google/uuid"
)

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

type AssignRequest struct {
	AssignedTo uuid.UUID `json:"assigned_to" validate:"required"`
}
