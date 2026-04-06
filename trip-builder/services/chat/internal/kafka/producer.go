package kafka

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type ChatProducer struct {
	producer *kafka.Producer
	log      *logger.Logger
}

func NewChatProducer(brokers []string, log *logger.Logger) *ChatProducer {
	log.Debug("Создание ChatProducer", "brokers", brokers)

	baseProducer := kafka.NewProducer(brokers, "chat", log)

	return &ChatProducer{
		producer: baseProducer,
		log:      log,
	}
}

// PublishChatMessage публикует событие о новом сообщении для уведомлений
func (p *ChatProducer) PublishChatMessage(ctx context.Context, eventID, userID uuid.UUID, text string) error {
	p.log.Debug("Подготовка события chat.message",
		"event_id", eventID,
		"user_id", userID,
	)

	kafkaEvent := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "chat.message",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"event_id": eventID.String(),
			"user_id":  userID.String(),
			"text":     text,
			"sent_at":  time.Now(),
		},
	}

	err := p.producer.PublishTo(ctx, "chat.message", kafkaEvent)
	if err != nil {
		p.log.Error("Ошибка публикации chat.message",
			"error", err,
			"event_id", eventID,
		)
		return err
	}

	p.log.Info("Событие chat.message опубликовано", "event_id", eventID)
	return nil
}

func (p *ChatProducer) Close() error {
	p.log.Debug("Закрытие ChatProducer")
	return p.producer.Close()
}
