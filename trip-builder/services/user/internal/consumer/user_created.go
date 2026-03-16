package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/segmentio/kafka-go"
)

type UserCreatedConsumer struct {
	reader         *kafka.Reader
	profileService *service.ProfileService
	log            *logger.Logger
}

func NewUserCreatedConsumer(brokers []string, topic string, groupID string, profileService *service.ProfileService, log *logger.Logger) *UserCreatedConsumer {
	log.Info("Создание UserCreatedConsumer",
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
	return &UserCreatedConsumer{
		reader:         reader,
		profileService: profileService,
		log:            log,
	}
}

func (c *UserCreatedConsumer) Start(ctx context.Context) {
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

func (c *UserCreatedConsumer) processMessage(ctx context.Context, msg kafka.Message) {
	log := c.log.With(
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
	)

	var base BaseEvent
	if err := json.Unmarshal(msg.Value, &base); err != nil {
		log.Error("Ошибка парсинга базового события", "error", err)
		return
	}

	var data UserCreatedData
	if err := json.Unmarshal(base.Data, &data); err != nil {
		log.Error("Ошибка парсинга данных user.created", "error", err)
		return
	}

	log.Debug("Обработка user.created", "user_id", data.UserID)

	if err := c.profileService.CreateProfile(ctx, data.UserID); err != nil {
		log.Error("Ошибка создания профиля", "error", err, "user_id", data.UserID)
		return
	}

	log.Info("Профиль обработан", "user_id", data.UserID)
}

func (c *UserCreatedConsumer) Close() error {
	return c.reader.Close()
}