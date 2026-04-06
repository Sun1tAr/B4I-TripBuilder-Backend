package kafka

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type MediaProducer struct {
	producer *kafka.Producer
	log      *logger.Logger
}

func NewMediaProducer(brokers []string, log *logger.Logger) *MediaProducer {
	log.Debug("Создание MediaProducer", "brokers", brokers)

	baseProducer := kafka.NewProducer(brokers, "media", log)

	return &MediaProducer{
		producer: baseProducer,
		log:      log,
	}
}

// PublishMediaUploaded публикует событие о загрузке медиа
func (p *MediaProducer) PublishMediaUploaded(ctx context.Context, mediaID, userID uuid.UUID, eventID, waypointID *uuid.UUID, purpose string) error {
	p.log.Debug("Подготовка события media.uploaded",
		"media_id", mediaID,
		"user_id", userID,
		"purpose", purpose,
	)

	data := map[string]interface{}{
		"media_id":    mediaID.String(),
		"user_id":     userID.String(),
		"purpose":     purpose,
		"uploaded_at": time.Now(),
	}
	if eventID != nil {
		data["event_id"] = eventID.String()
	}
	if waypointID != nil {
		data["waypoint_id"] = waypointID.String()
	}

	kafkaEvent := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "media.uploaded",
		Timestamp: time.Now(),
		Data:      data,
	}

	err := p.producer.PublishTo(ctx, "media.uploaded", kafkaEvent)
	if err != nil {
		p.log.Error("Ошибка публикации media.uploaded",
			"error", err,
			"media_id", mediaID,
		)
		return err
	}

	p.log.Info("Событие media.uploaded опубликовано", "media_id", mediaID)
	return nil
}

// Close закрывает продюсер
func (p *MediaProducer) Close() error {
	p.log.Debug("Закрытие MediaProducer")
	return p.producer.Close()
}
