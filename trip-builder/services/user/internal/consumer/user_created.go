package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/service"
	authdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/auth"
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
		Brokers:           brokers,
		Topic:             topic,
		GroupID:           groupID,
		MinBytes:          1, // НЕ ждём накопления
		MaxBytes:          10e6,
		MaxWait:           500 * time.Millisecond, // Чаще отправляем heartbeat
		CommitInterval:    5 * time.Second,        // Асинхронный коммит (не блокирует)
		SessionTimeout:    30 * time.Second,       // Время жизни сессии
		HeartbeatInterval: 3 * time.Second,        // Частота heartbeat
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

	c.log.Info("Получено сообщение", "value_preview", string(msg.Value[:min(100, len(msg.Value))]))

	// Парсим событие
	var event struct {
		EventID   string          `json:"event_id"`
		EventType string          `json:"event_type"`
		Timestamp time.Time       `json:"timestamp"`
		Data      json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(msg.Value, &event); err != nil {
		c.log.Error("Ошибка парсинга базового события", "error", err)
		return
	}

	// Парсим данные
	var data authdto.UserCreatedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		c.log.Error("Ошибка парсинга данных user.created", "error", err)
		return
	}

	c.log.Debug("Обработка user.created",
		"user_id", data.UserID,
		"email", data.Email,
		"first_name", data.FirstName,
		"last_name", data.LastName,
	)

	if err := c.profileService.CreateProfile(ctx, data.UserID, data.FirstName, data.LastName); err != nil {
		c.log.Error("Ошибка создания профиля", "error", err, "user_id", data.UserID)
		return
	}

	c.log.Info("Профиль обработан", "user_id", data.UserID)
}

func (c *UserCreatedConsumer) Close() error {
	return c.reader.Close()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
