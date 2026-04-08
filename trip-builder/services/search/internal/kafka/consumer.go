package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/segmentio/kafka-go"
)

type EventConsumer struct {
	reader        *kafka.Reader
	searchService *service.SearchService
	log           *logger.Logger
	topic         string
}

func NewEventConsumer(brokers []string, topic, groupID string, searchService *service.SearchService, log *logger.Logger) *EventConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3,
		MaxBytes: 10e6,
		MaxWait:  1 * time.Second,
	})
	return &EventConsumer{
		reader:        reader,
		searchService: searchService,
		log:           log,
		topic:         topic,
	}
}

func (c *EventConsumer) Start(ctx context.Context) {
	c.log.Info("Запуск consumer для топика", "topic", c.topic)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Остановка consumer", "topic", c.topic)
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
	var baseEvent struct {
		EventID   string          `json:"event_id"`
		EventType string          `json:"event_type"`
		Timestamp time.Time       `json:"timestamp"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(msg.Value, &baseEvent); err != nil {
		c.log.Error("Ошибка парсинга базового события", "error", err, "topic", msg.Topic)
		return
	}

	c.log.Debug("Получено событие", "event_type", baseEvent.EventType, "topic", c.topic)

	switch baseEvent.EventType {
	case "route.created", "route.updated":
		c.handleRouteUpsert(ctx, baseEvent.Data)
	case "route.deleted":
		c.handleRouteDelete(ctx, baseEvent.Data)
	case "event.created", "event.updated":
		c.handleEventUpsert(ctx, baseEvent.Data)
	case "event.deleted":
		c.handleEventDelete(ctx, baseEvent.Data)
	case "user.created", "user.updated":
		c.handleUserUpsert(ctx, baseEvent.Data)
	}
}

func (c *EventConsumer) handleRouteUpsert(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		RouteID     string    `json:"route_id"`
		UserID      string    `json:"user_id"`
		Title       string    `json:"title"`
		Country     string    `json:"country"`
		Description string    `json:"description"`
		IsPublic    bool      `json:"is_public"`
		TotalDays   int       `json:"total_days"`
		Tags        []string  `json:"tags"`
		CreatedAt   time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга route события", "error", err)
		return
	}

	doc := &models.RouteDocument{
		ID:          eventData.RouteID,
		Title:       eventData.Title,
		Description: eventData.Description,
		Country:     eventData.Country,
		Tags:        eventData.Tags,
		TotalDays:   eventData.TotalDays,
		UserID:      eventData.UserID,
		IsPublic:    eventData.IsPublic,
		CreatedAt:   eventData.CreatedAt,
	}
	c.searchService.IndexRoute(ctx, doc)
}

func (c *EventConsumer) handleRouteDelete(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		RouteID string `json:"route_id"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга route.deleted", "error", err)
		return
	}
	c.searchService.DeleteRoute(ctx, eventData.RouteID)
}

func (c *EventConsumer) handleEventUpsert(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		EventID      string    `json:"event_id"`
		RouteID      string    `json:"route_id"`
		Title        string    `json:"title"`
		StartDate    time.Time `json:"start_date"`
		IsPublic     bool      `json:"is_public"`
		Status       string    `json:"status"`
		Participants []string  `json:"participants"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга event события", "error", err)
		return
	}

	doc := &models.EventDocument{
		ID:             eventData.EventID,
		Title:          eventData.Title,
		RouteID:        eventData.RouteID,
		StartDate:      eventData.StartDate,
		IsPublic:       eventData.IsPublic,
		Status:         eventData.Status,
		ParticipantIDs: eventData.Participants,
	}
	c.searchService.IndexEvent(ctx, doc)
}

func (c *EventConsumer) handleEventDelete(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга event.deleted", "error", err)
		return
	}
	c.searchService.DeleteEvent(ctx, eventData.EventID)
}

func (c *EventConsumer) handleUserUpsert(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		UserID    string `json:"user_id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Bio       string `json:"bio"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга user события", "error", err)
		return
	}

	doc := &models.UserDocument{
		ID:        eventData.UserID,
		FirstName: eventData.FirstName,
		LastName:  eventData.LastName,
		Bio:       eventData.Bio,
	}
	c.searchService.IndexUser(ctx, doc)
}

func (c *EventConsumer) Close() error {
	return c.reader.Close()
}
