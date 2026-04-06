package kafka

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type EventProducer struct {
	producer *kafka.Producer
	cfg      *config.Config
	log      *logger.Logger
}

func NewEventProducer(cfg *config.Config, log *logger.Logger) *EventProducer {
	log.Debug("Создание EventProducer", "brokers", cfg.Kafka.Brokers)

	baseProducer := kafka.NewProducer(cfg.Kafka.Brokers, "event", log)

	return &EventProducer{
		producer: baseProducer,
		cfg:      cfg,
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

	err := p.producer.PublishTo(ctx, p.cfg.EventCreatedTopic, kafkaEvent)
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

	err := p.producer.PublishTo(ctx, p.cfg.EventUpdatedTopic, kafkaEvent)
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

	err := p.producer.PublishTo(ctx, p.cfg.EventCompletedTopic, kafkaEvent)
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

	err := p.producer.PublishTo(ctx, p.cfg.EventCancelledTopic, kafkaEvent)
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

	err := p.producer.PublishTo(ctx, p.cfg.WaypointVisitedTopic, kafkaEvent)
	if err != nil {
		p.log.Error("Ошибка публикации waypoint.visited", "error", err, "event_id", eventID, "waypoint_id", waypointID)
		return err
	}

	p.log.Info("Событие waypoint.visited опубликовано", "event_id", eventID, "waypoint_id", waypointID)
	return nil
}

// PublishParticipantJoined публикует событие о присоединении участника
func (p *EventProducer) PublishParticipantJoined(ctx context.Context, eventID, userID uuid.UUID) error {
	p.log.Debug("Подготовка события event.participant_joined",
		"event_id", eventID,
		"user_id", userID,
	)

	kafkaEvent := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "event.participant_joined",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"event_id":  eventID.String(),
			"user_id":   userID.String(),
			"joined_at": time.Now(),
		},
	}

	err := p.producer.PublishTo(ctx, p.cfg.ParticipantJoinedTopic, kafkaEvent)
	if err != nil {
		p.log.Error("Ошибка публикации event.participant_joined",
			"error", err,
			"event_id", eventID,
		)
		return err
	}

	p.log.Info("Событие event.participant_joined опубликовано", "event_id", eventID, "user_id", userID)
	return nil
}

// Close закрывает продюсер
func (p *EventProducer) Close() error {
	p.log.Debug("Закрытие EventProducer")
	return p.producer.Close()
}

// PublishEventDeleted публикует событие об удалении поездки
func (p *EventProducer) PublishEventDeleted(ctx context.Context, eventID, ownerID uuid.UUID) error {
	p.log.Debug("Подготовка события event.deleted",
		"event_id", eventID,
		"owner_id", ownerID,
	)

	kafkaEvent := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "event.deleted",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"event_id":   eventID.String(),
			"owner_id":   ownerID.String(),
			"deleted_at": time.Now(),
		},
	}

	err := p.producer.PublishTo(ctx, p.cfg.EventDeletedTopic, kafkaEvent)
	if err != nil {
		p.log.Error("Ошибка публикации event.deleted",
			"error", err,
			"event_id", eventID,
		)
		return err
	}

	p.log.Info("Событие event.deleted опубликовано", "event_id", eventID)
	return nil
}
