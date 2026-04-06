package kafka

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type UserProducer struct {
	producer *kafka.Producer
	log      *logger.Logger
}

func NewUserProducer(brokers []string, log *logger.Logger) *UserProducer {
	prod := kafka.NewProducer(brokers, "user", log)
	return &UserProducer{
		producer: prod,
		log:      log,
	}
}

func (p *UserProducer) PublishAchievement(ctx context.Context, userID uuid.UUID, achievementType, title string) error {
	event := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "user.achievement_earned",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"user_id":          userID.String(),
			"achievement_type": achievementType,
			"title":            title,
		},
	}
	return p.producer.Publish(ctx, event)
}

func (p *UserProducer) PublishUserUpdated(ctx context.Context, userID uuid.UUID, firstName, lastName, avatarURL string) error {
	p.log.Debug("Подготовка события user.updated",
		"user_id", userID,
		"first_name", firstName,
		"last_name", lastName,
	)

	event := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "user.updated",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"user_id":    userID.String(),
			"first_name": firstName,
			"last_name":  lastName,
			"avatar_url": avatarURL,
			"updated_at": time.Now(),
		},
	}

	if err := p.producer.Publish(ctx, event); err != nil {
		p.log.Error("Ошибка публикации user.updated",
			"error", err,
			"user_id", userID,
		)
		return err
	}

	p.log.Info("Событие user.updated опубликовано", "user_id", userID)
	return nil
}

func (p *UserProducer) Close() error {
	return p.producer.Close()
}
