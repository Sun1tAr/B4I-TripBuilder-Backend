package task

import (
	"time"

	"github.com/google/uuid"
)

type TaskCompletedEvent struct {
	EventID   string            `json:"event_id"`
	EventType string            `json:"event_type"`
	Timestamp time.Time         `json:"timestamp"`
	Data      TaskCompletedData `json:"data"`
}

type TaskCompletedData struct {
	TaskID      uuid.UUID `json:"task_id"`
	EventID     uuid.UUID `json:"event_id"`
	UserID      uuid.UUID `json:"user_id"`
	Title       string    `json:"title"`
	CompletedAt time.Time `json:"completed_at"`
}

type EventCreatedData struct {
	EventID   uuid.UUID `json:"event_id"`
	RouteID   uuid.UUID `json:"route_id"`
	OwnerID   uuid.UUID `json:"owner_id"`
	Title     string    `json:"title"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

type EventUpdatedData struct {
	EventID   uuid.UUID `json:"event_id"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

// TaskAssignedEvent событие назначения задачи
type TaskAssignedEvent struct {
	EventID   string           `json:"event_id"`
	EventType string           `json:"event_type"`
	Timestamp time.Time        `json:"timestamp"`
	Data      TaskAssignedData `json:"data"`
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
