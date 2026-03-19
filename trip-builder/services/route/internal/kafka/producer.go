package kafka

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type RouteProducer struct {
	producer *kafka.Producer
	log      *logger.Logger
}

func NewRouteProducer(brokers []string, log *logger.Logger) *RouteProducer {
	log.Debug("Создание RouteProducer", "brokers", brokers)

	// Создаём базовый продюсер с дефолтным топиком (можно пустой или любой)
	// Будем использовать PublishTo для отправки в разные топики
	baseProducer := kafka.NewProducer(brokers, "route", log)

	return &RouteProducer{
		producer: baseProducer,
		log:      log,
	}
}

// PublishRouteCreated публикует событие о создании маршрута
func (p *RouteProducer) PublishRouteCreated(ctx context.Context, routeID, userID uuid.UUID, title, country string, isPublic bool, totalDays, totalHours int) error {
	p.log.Debug("Подготовка события route.created",
		"route_id", routeID,
		"user_id", userID,
		"title", title,
	)

	event := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "route.created",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"route_id":    routeID.String(),
			"user_id":     userID.String(),
			"title":       title,
			"country":     country,
			"is_public":   isPublic,
			"total_days":  totalDays,
			"total_hours": totalHours,
			"created_at":  time.Now(),
		},
	}

	// Используем новый метод PublishTo
	err := p.producer.PublishTo(ctx, "route.created", event)
	if err != nil {
		p.log.Error("Ошибка публикации route.created",
			"error", err,
			"route_id", routeID,
		)
		return err
	}

	p.log.Info("Событие route.created опубликовано",
		"route_id", routeID,
		"user_id", userID,
	)
	return nil
}

// PublishRouteUpdated публикует событие об обновлении маршрута
func (p *RouteProducer) PublishRouteUpdated(ctx context.Context, routeID, userID uuid.UUID, title, country string, isPublic bool) error {
	p.log.Debug("Подготовка события route.updated",
		"route_id", routeID,
		"user_id", userID,
	)

	event := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "route.updated",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"route_id":   routeID.String(),
			"user_id":    userID.String(),
			"title":      title,
			"country":    country,
			"is_public":  isPublic,
			"updated_at": time.Now(),
		},
	}

	err := p.producer.PublishTo(ctx, "route.updated", event)
	if err != nil {
		p.log.Error("Ошибка публикации route.updated",
			"error", err,
			"route_id", routeID,
		)
		return err
	}

	p.log.Info("Событие route.updated опубликовано",
		"route_id", routeID,
		"user_id", userID,
	)
	return nil
}

// PublishRouteDeleted публикует событие об удалении маршрута
func (p *RouteProducer) PublishRouteDeleted(ctx context.Context, routeID, userID uuid.UUID) error {
	p.log.Debug("Подготовка события route.deleted",
		"route_id", routeID,
		"user_id", userID,
	)

	event := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "route.deleted",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"route_id":   routeID.String(),
			"user_id":    userID.String(),
			"deleted_at": time.Now(),
		},
	}

	err := p.producer.PublishTo(ctx, "route.deleted", event)
	if err != nil {
		p.log.Error("Ошибка публикации route.deleted",
			"error", err,
			"route_id", routeID,
		)
		return err
	}

	p.log.Info("Событие route.deleted опубликовано",
		"route_id", routeID,
		"user_id", userID,
	)
	return nil
}

// Close закрывает продюсер
func (p *RouteProducer) Close() error {
	p.log.Debug("Закрытие RouteProducer")
	return p.producer.Close()
}
