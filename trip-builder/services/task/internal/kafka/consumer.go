package kafka

import (
	"context"
	"encoding/json"
	"time"

	taskdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/task"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// TaskServiceInterface - интерфейс для методов, вызываемых из consumer
type TaskServiceInterface interface {
	GenerateTasksFromTemplates(ctx context.Context, eventID uuid.UUID, startDate time.Time, createdBy uuid.UUID) ([]interface{}, error)
	RecalculateDueDates(ctx context.Context, eventID uuid.UUID, newStartDate time.Time) error
	CancelEventTasks(ctx context.Context, eventID uuid.UUID) error
}

type EventConsumer struct {
	reader                   *kafka.Reader
	taskService              TaskServiceInterface
	log                      *logger.Logger
	generateTemplatesEnabled bool
}

func NewEventConsumer(brokers []string, topic, groupID string, taskService TaskServiceInterface, log *logger.Logger, generateTemplatesEnabled bool) *EventConsumer {
	log.Debug("Создание EventConsumer",
		"brokers", brokers,
		"topic", topic,
		"group_id", groupID,
		"generate_templates_enabled", generateTemplatesEnabled,
	)

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3,
		MaxBytes: 10e6,
		MaxWait:  1 * time.Second,
	})

	return &EventConsumer{
		reader:                   reader,
		taskService:              taskService,
		log:                      log,
		generateTemplatesEnabled: generateTemplatesEnabled,
	}
}

func (c *EventConsumer) Start(ctx context.Context) {
	c.log.Info("Запуск consumer для топика", "topic", c.reader.Config().Topic)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Остановка consumer", "topic", c.reader.Config().Topic)
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				c.log.Error("Ошибка чтения сообщения", "error", err)
				continue
			}

			c.processMessage(ctx, msg)
		}
	}
}

func (c *EventConsumer) processMessage(ctx context.Context, msg kafka.Message) {
	// Не создаем новый логгер с полями, используем существующий
	var baseEvent struct {
		EventID   string          `json:"event_id"`
		EventType string          `json:"event_type"`
		Timestamp time.Time       `json:"timestamp"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(msg.Value, &baseEvent); err != nil {
		c.log.Error("Ошибка парсинга базового события", "error", err)
		return
	}

	c.log.Debug("Получено событие", "event_type", baseEvent.EventType, "topic", msg.Topic, "partition", msg.Partition, "offset", msg.Offset)

	switch baseEvent.EventType {
	case "event.created":
		c.handleEventCreated(ctx, baseEvent.Data)
	case "event.updated":
		c.handleEventUpdated(ctx, baseEvent.Data)
	case "event.cancelled":
		c.handleEventCancelled(ctx, baseEvent.Data)
	default:
		c.log.Debug("Пропуск события другого типа", "event_type", baseEvent.EventType)
	}
}

func (c *EventConsumer) handleEventCreated(ctx context.Context, data json.RawMessage) {
	var eventData taskdto.EventCreatedData
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга event.created", "error", err)
		return
	}

	c.log.Debug("Обработка event.created",
		"event_id", eventData.EventID,
		"start_date", eventData.StartDate,
		"owner_id", eventData.OwnerID,
		"generate_templates_enabled", c.generateTemplatesEnabled,
	)

	// Генерируем задачи по шаблонам только если включено
	if !c.generateTemplatesEnabled {
		c.log.Debug("Генерация шаблонных задач отключена", "event_id", eventData.EventID)
		return
	}

	tasks, err := c.taskService.GenerateTasksFromTemplates(ctx, eventData.EventID, eventData.StartDate, eventData.OwnerID)
	if err != nil {
		c.log.Error("Ошибка генерации задач", "error", err, "event_id", eventData.EventID)
		return
	}

	c.log.Info("Задачи сгенерированы для события", "event_id", eventData.EventID, "tasks_count", len(tasks))
}

func (c *EventConsumer) handleEventUpdated(ctx context.Context, data json.RawMessage) {
	var eventData taskdto.EventUpdatedData
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга event.updated", "error", err)
		return
	}

	c.log.Debug("Обработка event.updated",
		"event_id", eventData.EventID,
		"start_date", eventData.StartDate,
	)

	// Пересчитываем дедлайны задач
	if err := c.taskService.RecalculateDueDates(ctx, eventData.EventID, eventData.StartDate); err != nil {
		c.log.Error("Ошибка пересчёта дедлайнов", "error", err, "event_id", eventData.EventID)
		return
	}

	c.log.Info("Дедлайны задач пересчитаны", "event_id", eventData.EventID)
}

func (c *EventConsumer) handleEventCancelled(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		EventID uuid.UUID `json:"event_id"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга event.cancelled", "error", err)
		return
	}

	c.log.Debug("Обработка event.cancelled", "event_id", eventData.EventID)

	if err := c.taskService.CancelEventTasks(ctx, eventData.EventID); err != nil {
		c.log.Error("Ошибка отмены задач", "error", err, "event_id", eventData.EventID)
		return
	}

	c.log.Info("Задачи отменены", "event_id", eventData.EventID)
}

func (c *EventConsumer) Close() error {
	return c.reader.Close()
}
