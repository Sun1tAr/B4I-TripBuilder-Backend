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
	log.Debug("Создание EventConsumer",
		"brokers", brokers,
		"topic", topic,
		"group_id", groupID,
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
		c.log.Error("Ошибка парсинга базового события", "error", err)
		return
	}

	c.log.Debug("Получено событие", "event_type", baseEvent.EventType, "topic", msg.Topic)

	switch baseEvent.EventType {
	// Маршруты
	case "route.created":
		c.handleRouteCreated(ctx, baseEvent.Data)
	case "route.updated":
		c.handleRouteUpdated(ctx, baseEvent.Data)
	case "route.deleted":
		c.handleRouteDeleted(ctx, baseEvent.Data)

	// События (поездки)
	case "event.created":
		c.handleEventCreated(ctx, baseEvent.Data)
	case "event.updated":
		c.handleEventUpdated(ctx, baseEvent.Data)
	case "event.deleted":
		c.handleEventDeleted(ctx, baseEvent.Data)

	// Пользователи
	case "user.created":
		c.handleUserCreated(ctx, baseEvent.Data)
	case "user.updated":
		c.handleUserUpdated(ctx, baseEvent.Data)

	default:
		c.log.Debug("Пропуск события неизвестного типа", "event_type", baseEvent.EventType)
	}
}

func (c *EventConsumer) handleRouteCreated(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		RouteID     string    `json:"route_id"`
		UserID      string    `json:"user_id"`
		Title       string    `json:"title"`
		Country     string    `json:"country"`
		Description string    `json:"description"`
		IsPublic    bool      `json:"is_public"`
		TotalDays   int       `json:"total_days"`
		TotalHours  int       `json:"total_hours"`
		Tags        []string  `json:"tags"`
		CreatedAt   time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга route.created", "error", err)
		return
	}

	c.log.Debug("Обработка route.created", "route_id", eventData.RouteID, "title", eventData.Title, "is_public", eventData.IsPublic)

	doc := &models.RouteDocument{
		ID:          eventData.RouteID,
		Title:       eventData.Title,
		Description: eventData.Description,
		Country:     eventData.Country,
		Tags:        eventData.Tags,
		TotalDays:   eventData.TotalDays,
		UserID:      eventData.UserID,
		LikesCount:  0,
		IsPublic:    eventData.IsPublic,
		CreatedAt:   eventData.CreatedAt,
		Suggest:     c.buildRouteSuggest(eventData.Title, eventData.Country, eventData.Tags),
	}

	if err := c.searchService.IndexRoute(ctx, doc); err != nil {
		c.log.Error("Ошибка индексации маршрута", "error", err, "route_id", eventData.RouteID)
	}
}

func (c *EventConsumer) handleRouteUpdated(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		RouteID     string    `json:"route_id"`
		UserID      string    `json:"user_id"`
		Title       string    `json:"title"`
		Country     string    `json:"country"`
		Description string    `json:"description"`
		IsPublic    bool      `json:"is_public"`
		Tags        []string  `json:"tags"`
		UpdatedAt   time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга route.updated", "error", err)
		return
	}

	c.log.Debug("Обработка route.updated", "route_id", eventData.RouteID, "title", eventData.Title, "is_public", eventData.IsPublic)

	doc := &models.RouteDocument{
		ID:          eventData.RouteID,
		Title:       eventData.Title,
		Description: eventData.Description,
		Country:     eventData.Country,
		Tags:        eventData.Tags,
		UserID:      eventData.UserID,
		IsPublic:    eventData.IsPublic,
		Suggest:     c.buildRouteSuggest(eventData.Title, eventData.Country, eventData.Tags),
	}

	if err := c.searchService.IndexRoute(ctx, doc); err != nil {
		c.log.Error("Ошибка обновления маршрута в индексе", "error", err, "route_id", eventData.RouteID)
	}
}

func (c *EventConsumer) handleRouteDeleted(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		RouteID string `json:"route_id"`
		UserID  string `json:"user_id"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга route.deleted", "error", err)
		return
	}

	c.log.Debug("Обработка route.deleted", "route_id", eventData.RouteID)

	if err := c.searchService.DeleteRoute(ctx, eventData.RouteID); err != nil {
		c.log.Error("Ошибка удаления маршрута из индекса", "error", err, "route_id", eventData.RouteID)
	}
}

func (c *EventConsumer) handleEventCreated(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		EventID      string    `json:"event_id"`
		RouteID      string    `json:"route_id"`
		Title        string    `json:"title"`
		StartDate    time.Time `json:"start_date"`
		IsPublic     bool      `json:"is_public"`
		Participants []string  `json:"participants"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга event.created", "error", err)
		return
	}

	c.log.Debug("Обработка event.created", "event_id", eventData.EventID, "title", eventData.Title, "is_public", eventData.IsPublic)

	doc := &models.EventDocument{
		ID:             eventData.EventID,
		Title:          eventData.Title,
		RouteID:        eventData.RouteID,
		StartDate:      eventData.StartDate,
		Status:         "planned",
		ParticipantIDs: eventData.Participants,
		IsPublic:       eventData.IsPublic,
		Suggest:        []string{eventData.Title},
	}

	if err := c.searchService.IndexEvent(ctx, doc); err != nil {
		c.log.Error("Ошибка индексации события", "error", err, "event_id", eventData.EventID)
	}
}

func (c *EventConsumer) handleEventUpdated(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		EventID   string    `json:"event_id"`
		Title     string    `json:"title"`
		StartDate time.Time `json:"start_date"`
		IsPublic  bool      `json:"is_public"`
		Status    string    `json:"status"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга event.updated", "error", err)
		return
	}

	c.log.Debug("Обработка event.updated", "event_id", eventData.EventID, "title", eventData.Title, "is_public", eventData.IsPublic, "status", eventData.Status)

	doc := &models.EventDocument{
		ID:        eventData.EventID,
		Title:     eventData.Title,
		StartDate: eventData.StartDate,
		IsPublic:  eventData.IsPublic,
		Status:    eventData.Status,
		Suggest:   []string{eventData.Title},
	}

	if err := c.searchService.IndexEvent(ctx, doc); err != nil {
		c.log.Error("Ошибка обновления события в индексе", "error", err, "event_id", eventData.EventID)
	}
}

func (c *EventConsumer) handleEventDeleted(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга event.deleted", "error", err)
		return
	}

	c.log.Debug("Обработка event.deleted", "event_id", eventData.EventID)

	if err := c.searchService.DeleteEvent(ctx, eventData.EventID); err != nil {
		c.log.Error("Ошибка удаления события из индекса", "error", err, "event_id", eventData.EventID)
	}
}

func (c *EventConsumer) handleUserCreated(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		UserID    string `json:"user_id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга user.created", "error", err)
		return
	}

	c.log.Debug("Обработка user.created", "user_id", eventData.UserID, "first_name", eventData.FirstName, "last_name", eventData.LastName)

	doc := &models.UserDocument{
		ID:        eventData.UserID,
		FirstName: eventData.FirstName,
		LastName:  eventData.LastName,
		Suggest:   []string{eventData.FirstName, eventData.LastName},
	}

	if err := c.searchService.IndexUser(ctx, doc); err != nil {
		c.log.Error("Ошибка индексации пользователя", "error", err, "user_id", eventData.UserID)
	}
}

func (c *EventConsumer) handleUserUpdated(ctx context.Context, data json.RawMessage) {
	var eventData struct {
		UserID    string `json:"user_id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		AvatarURL string `json:"avatar_url"`
		Bio       string `json:"bio"`
	}
	if err := json.Unmarshal(data, &eventData); err != nil {
		c.log.Error("Ошибка парсинга user.updated", "error", err)
		return
	}

	c.log.Debug("Обработка user.updated", "user_id", eventData.UserID, "first_name", eventData.FirstName, "last_name", eventData.LastName)

	doc := &models.UserDocument{
		ID:        eventData.UserID,
		FirstName: eventData.FirstName,
		LastName:  eventData.LastName,
		Bio:       eventData.Bio,
		Suggest:   []string{eventData.FirstName, eventData.LastName},
	}

	if err := c.searchService.IndexUser(ctx, doc); err != nil {
		c.log.Error("Ошибка обновления пользователя в индексе", "error", err, "user_id", eventData.UserID)
	}
}

func (c *EventConsumer) buildRouteSuggest(title, country string, tags []string) []string {
	suggest := []string{title}
	if country != "" {
		suggest = append(suggest, country)
	}
	suggest = append(suggest, tags...)
	return suggest
}

func (c *EventConsumer) Close() error {
	return c.reader.Close()
}
