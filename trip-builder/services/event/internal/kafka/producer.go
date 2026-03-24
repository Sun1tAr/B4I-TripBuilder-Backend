package kafka

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type EventProducer struct {
	producer *kafka.Producer
	log      *logger.Logger
}

func NewEventProducer(brokers []string, log *logger.Logger) *EventProducer {
	log.Debug("Создание EventProducer", "brokers", brokers)

	// Создаём базовый продюсер с дефолтным топиком
	baseProducer := kafka.NewProducer(brokers, "event", log)

	return &EventProducer{
		producer: baseProducer,
		log:      log,
	}
}

// PublishEventCreated публикует событие о создании поездки
func (p *EventProducer) PublishEventCreated(ctx context.Context, eventID, routeID, ownerID uuid.UUID, title string, startDate, endDate time.Time, participants []uuid.UUID) error {
	p.log.Debug("Подготовка события event.created",
		"event_id", eventID,
		"route_id", routeID,
		"owner_id", ownerID,
	)

	kafkaEvent := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "event.created",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"event_id":     eventID.String(),
			"route_id":     routeID.String(),
			"owner_id":     ownerID.String(),
			"title":        title,
			"start_date":   startDate,
			"end_date":     endDate,
			"participants": participants,
			"created_at":   time.Now(),
		},
	}

	err := p.producer.PublishTo(ctx, "event.created", kafkaEvent)
	if err != nil {
		p.log.Error("Ошибка публикации event.created",
			"error", err,
			"event_id", eventID,
		)
		return err
	}

	p.log.Info("Событие event.created опубликовано", "event_id", eventID)
	return nil
}

// PublishEventUpdated публикует событие об обновлении поездки
func (p *EventProducer) PublishEventUpdated(ctx context.Context, eventID, ownerID uuid.UUID, title string, startDate, endDate time.Time, isPublic bool) error {
	p.log.Debug("Подготовка события event.updated", "event_id", eventID)

	kafkaEvent := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "event.updated",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"event_id":   eventID.String(),
			"owner_id":   ownerID.String(),
			"title":      title,
			"start_date": startDate,
			"end_date":   endDate,
			"is_public":  isPublic,
			"updated_at": time.Now(),
		},
	}

	err := p.producer.PublishTo(ctx, "event.updated", kafkaEvent)
	if err != nil {
		p.log.Error("Ошибка публикации event.updated", "error", err, "event_id", eventID)
		return err
	}

	p.log.Info("Событие event.updated опубликовано", "event_id", eventID)
	return nil
}

// PublishEventCompleted публикует событие о завершении поездки
func (p *EventProducer) PublishEventCompleted(ctx context.Context, eventID, routeID, ownerID uuid.UUID, participants []uuid.UUID, countries []string) error {
	p.log.Debug("Подготовка события event.completed", "event_id", eventID)

	kafkaEvent := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "event.completed",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"event_id":          eventID.String(),
			"route_id":          routeID.String(),
			"owner_id":          ownerID.String(),
			"participants":      participants,
			"countries_visited": countries,
			"completed_at":      time.Now(),
		},
	}

	err := p.producer.PublishTo(ctx, "event.completed", kafkaEvent)
	if err != nil {
		p.log.Error("Ошибка публикации event.completed", "error", err, "event_id", eventID)
		return err
	}

	p.log.Info("Событие event.completed опубликовано", "event_id", eventID)
	return nil
}

// PublishEventCancelled публикует событие об отмене поездки
func (p *EventProducer) PublishEventCancelled(ctx context.Context, eventID, ownerID uuid.UUID) error {
	p.log.Debug("Подготовка события event.cancelled", "event_id", eventID)

	kafkaEvent := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "event.cancelled",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"event_id":     eventID.String(),
			"owner_id":     ownerID.String(),
			"cancelled_at": time.Now(),
		},
	}

	err := p.producer.PublishTo(ctx, "event.cancelled", kafkaEvent)
	if err != nil {
		p.log.Error("Ошибка публикации event.cancelled", "error", err, "event_id", eventID)
		return err
	}

	p.log.Info("Событие event.cancelled опубликовано", "event_id", eventID)
	return nil
}

// PublishWaypointVisited публикует событие о посещении точки
func (p *EventProducer) PublishWaypointVisited(ctx context.Context, eventID, waypointID, userID uuid.UUID, city string) error {
	p.log.Debug("Подготовка события waypoint.visited", "event_id", eventID, "waypoint_id", waypointID)

	kafkaEvent := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "waypoint.visited",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"event_id":    eventID.String(),
			"waypoint_id": waypointID.String(),
			"user_id":     userID.String(),
			"city":        city,
			"visited_at":  time.Now(),
		},
	}

	err := p.producer.PublishTo(ctx, "waypoint.visited", kafkaEvent)
	if err != nil {
		p.log.Error("Ошибка публикации waypoint.visited", "error", err, "event_id", eventID, "waypoint_id", waypointID)
		return err
	}

	p.log.Info("Событие waypoint.visited опубликовано", "event_id", eventID, "waypoint_id", waypointID)
	return nil
}

// Close закрывает продюсер
func (p *EventProducer) Close() error {
	p.log.Debug("Закрытие EventProducer")
	return p.producer.Close()
}
