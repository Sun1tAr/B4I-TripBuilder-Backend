package kafka

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type AchievementProducer struct {
	producer *kafka.Producer
	log      *logger.Logger
}

func NewAchievementProducer(brokers []string, log *logger.Logger) *AchievementProducer {
	prod := kafka.NewProducer(brokers, "user.achievement_earned", log)
	return &AchievementProducer{
		producer: prod,
		log:      log,
	}
}

func (p *AchievementProducer) PublishAchievement(ctx context.Context, userID uuid.UUID, achievementType, title string) error {
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

func (p *AchievementProducer) Close() error {
	return p.producer.Close()
}