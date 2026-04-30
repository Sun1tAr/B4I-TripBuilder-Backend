package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type MessageType string

const (
	MessageTypeText              MessageType = "text"
	MessageTypeSystem            MessageType = "system"
	MessageTypeTaskCompleted     MessageType = "task_completed"
	MessageTypeMediaUploaded     MessageType = "media_uploaded"
	MessageTypeWaypointVisited   MessageType = "waypoint_visited"
	MessageTypeParticipantJoined MessageType = "participant_joined"
)

type MessageMetadata map[string]interface{}

func (m MessageMetadata) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *MessageMetadata) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, m)
}

type Message struct {
	ID        uuid.UUID       `json:"id"`
	EventID   uuid.UUID       `json:"event_id"`
	UserID    uuid.UUID       `json:"user_id"`
	Text      string          `json:"text"`
	Type      MessageType     `json:"type"`
	Metadata  MessageMetadata `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt *time.Time      `json:"updated_at,omitempty"`
}

type ReadReceipt struct {
	MessageID uuid.UUID `json:"message_id"`
	UserID    uuid.UUID `json:"user_id"`
	ReadAt    time.Time `json:"read_at"`
}

// Request/Response DTO
type SendMessageRequest struct {
	Text string `json:"text" validate:"required,max=5000"`
}

type UpdateMessageRequest struct {
	Text string `json:"text" validate:"required,max=5000"`
}

type ListMessagesRequest struct {
	Page int `json:"page"`
	Size int `json:"size"`
}

type MessageResponse struct {
	ID        uuid.UUID       `json:"id"`
	EventID   uuid.UUID       `json:"event_id"`
	UserID    uuid.UUID       `json:"user_id"`
	Text      string          `json:"text"`
	Type      MessageType     `json:"type"`
	Metadata  MessageMetadata `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt *time.Time      `json:"updated_at,omitempty"`
}

type UnreadCountResponse struct {
	Count int64 `json:"count"`
}
