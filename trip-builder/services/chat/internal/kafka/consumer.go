package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

type SystemEventConsumer struct {
	reader      *kafka.Reader
	chatService *service.ChatService
	log         *logger.Logger
	topic       string
}

func NewSystemEventConsumer(brokers []string, topic, groupID string, chatService *service.ChatService, log *logger.Logger) *SystemEventConsumer {
	log.Debug("Создание SystemEventConsumer",
		"brokers", brokers,
		"topic", topic,
		"group_id", groupID,
	)
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:           brokers,
		Topic:             topic,
		GroupID:           groupID,
		MinBytes:          1, // НЕ ждём накопления
		MaxBytes:          10e6,
		MaxWait:           500 * time.Millisecond, // Чаще отправляем heartbeat
		CommitInterval:    5 * time.Second,        // Асинхронный коммит (не блокирует)
		SessionTimeout:    30 * time.Second,       // Время жизни сессии
		HeartbeatInterval: 3 * time.Second,        // Частота heartbeat
	})

	return &SystemEventConsumer{
		reader:      reader,
		chatService: chatService,
		log:         log,
		topic:       topic,
	}
}

func (c *SystemEventConsumer) Start(ctx context.Context) {
	c.log.Info("Запуск consumer для топика", "topic", c.topic)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Остановка consumer", "topic", c.topic)
			return
		default:
			msg, err := c.reader.FetchMessage(ctx) // Используем FetchMessage вместо ReadMessage
			if err != nil {
				// Если reader закрыт, выходим
				if errors.Is(err, context.Canceled) {
					return
				}
				c.log.Error("Ошибка чтения сообщения", "error", err)
				time.Sleep(time.Second) // Не спамим ошибками
				continue
			}

			c.processMessage(ctx, msg)

			// Ручной коммит
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.log.Error("Ошибка коммита", "error", err)
			}
		}
	}
}

func (c *SystemEventConsumer) processMessage(ctx context.Context, msg kafka.Message) {
	var baseEvent struct {
		EventID   string          `json:"event_id"`
		EventType string          `json:"event_type"`
		Timestamp time.Time       `json:"timestamp"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(msg.Value, &baseEvent); err != nil {
		c.log.Error("Ошибка парсинга базового события", "error", err,
			"topic", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
		)
		return
	}

	c.log.Debug("Получено событие", "event_type", baseEvent.EventType,
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
	)

	switch baseEvent.EventType {
	case "waypoint.visited":
		c.handleWaypointVisited(ctx, baseEvent.Data)
	case "task.completed":
		c.handleTaskCompleted(ctx, baseEvent.Data)
	case "media.uploaded":
		c.handleMediaUploaded(ctx, baseEvent.Data)
	case "event.participant_joined":
		c.handleParticipantJoined(ctx, baseEvent.Data)
	default:
		c.log.Debug("Пропуск события другого типа", "event_type", baseEvent.EventType)
	}
}

func (c *SystemEventConsumer) handleWaypointVisited(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		EventID    uuid.UUID `json:"event_id"`
		WaypointID uuid.UUID `json:"waypoint_id"`
		UserID     uuid.UUID `json:"user_id"`
		City       string    `json:"city"`
		VisitedAt  time.Time `json:"visited_at"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга waypoint.visited", "error", err)
		return
	}

	c.log.Debug("Обработка waypoint.visited",
		"event_id", eventData.EventID,
		"user_id", eventData.UserID,
		"city", eventData.City,
	)

	text := fmt.Sprintf("Пользователь %s посетил точку: %s", eventData.UserID, eventData.City)

	_, err := c.chatService.SaveSystemMessage(ctx, eventData.EventID, models.MessageTypeWaypointVisited, text, map[string]interface{}{
		"user_id":     eventData.UserID,
		"waypoint_id": eventData.WaypointID,
		"city":        eventData.City,
	})
	if err != nil {
		c.log.Error("Ошибка сохранения системного сообщения", "error", err)
		return
	}

	c.log.Info("Системное сообщение о посещении точки сохранено", "event_id", eventData.EventID)
}

func (c *SystemEventConsumer) handleTaskCompleted(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		TaskID      uuid.UUID `json:"task_id"`
		EventID     uuid.UUID `json:"event_id"`
		UserID      uuid.UUID `json:"user_id"`
		Title       string    `json:"title"`
		CompletedAt time.Time `json:"completed_at"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга task.completed", "error", err)
		return
	}

	c.log.Debug("Обработка task.completed",
		"event_id", eventData.EventID,
		"user_id", eventData.UserID,
		"task_title", eventData.Title,
	)

	text := fmt.Sprintf("Пользователь %s выполнил задачу: %s", eventData.UserID, eventData.Title)

	_, err := c.chatService.SaveSystemMessage(ctx, eventData.EventID, models.MessageTypeTaskCreated, text, map[string]interface{}{
		"user_id": eventData.UserID,
		"task_id": eventData.TaskID,
		"title":   eventData.Title,
	})
	if err != nil {
		c.log.Error("Ошибка сохранения системного сообщения", "error", err)
		return
	}

	c.log.Info("Системное сообщение о выполнении задачи сохранено", "event_id", eventData.EventID)
}

func (c *SystemEventConsumer) handleMediaUploaded(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		MediaID    uuid.UUID  `json:"media_id"`
		EventID    uuid.UUID  `json:"event_id"`
		WaypointID *uuid.UUID `json:"waypoint_id,omitempty"`
		UserID     uuid.UUID  `json:"user_id"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга media.uploaded", "error", err)
		return
	}

	c.log.Debug("Обработка media.uploaded",
		"event_id", eventData.EventID,
		"user_id", eventData.UserID,
	)

	text := fmt.Sprintf("Пользователь %s загрузил новое фото", eventData.UserID)

	metadata := map[string]interface{}{
		"user_id":  eventData.UserID,
		"media_id": eventData.MediaID,
	}
	if eventData.WaypointID != nil {
		metadata["waypoint_id"] = eventData.WaypointID
		text = fmt.Sprintf("Пользователь %s загрузил фото к точке", eventData.UserID)
	}

	_, err := c.chatService.SaveSystemMessage(ctx, eventData.EventID, models.MessageTypeMediaUploaded, text, metadata)
	if err != nil {
		c.log.Error("Ошибка сохранения системного сообщения", "error", err)
		return
	}

	c.log.Info("Системное сообщение о загрузке медиа сохранено", "event_id", eventData.EventID)
}

func (c *SystemEventConsumer) handleParticipantJoined(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		EventID uuid.UUID `json:"event_id"`
		UserID  uuid.UUID `json:"user_id"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга event.participant_joined", "error", err)
		return
	}

	c.log.Debug("Обработка participant_joined",
		"event_id", eventData.EventID,
		"user_id", eventData.UserID,
	)

	text := fmt.Sprintf("Пользователь %s присоединился к поездке", eventData.UserID)

	_, err := c.chatService.SaveSystemMessage(ctx, eventData.EventID, models.MessageTypeParticipantJoined, text, map[string]interface{}{
		"user_id": eventData.UserID,
	})
	if err != nil {
		c.log.Error("Ошибка сохранения системного сообщения", "error", err)
		return
	}

	c.log.Info("Системное сообщение о присоединении участника сохранено", "event_id", eventData.EventID)
}

func (c *SystemEventConsumer) Close() error {
	return c.reader.Close()
}
