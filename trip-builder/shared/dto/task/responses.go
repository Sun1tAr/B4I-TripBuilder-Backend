package task

import (
	"time"

	"github.com/google/uuid"
)

type TaskResponse struct {
	ID          uuid.UUID  `json:"id"`
	EventID     uuid.UUID  `json:"event_id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	AssignedTo  *uuid.UUID `json:"assigned_to,omitempty"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	Priority    string     `json:"priority"`
	Status      string     `json:"status"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type TaskDetailResponse struct {
	TaskResponse
	Comments []TaskCommentResponse `json:"comments,omitempty"`
}

type TaskCommentResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskTemplateResponse struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	TriggerDays int       `json:"trigger_days"`
	Priority    string    `json:"priority"`
}

type ListTasksResponse struct {
	Items      []TaskResponse `json:"items"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	Size       int            `json:"size"`
	TotalPages int            `json:"total_pages"`
}

type MessageResponse struct {
	Message string `json:"message"`
}
