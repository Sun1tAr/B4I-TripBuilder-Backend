package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/segmentio/kafka-go"
)

type UserWelcomeConsumer struct {
	reader              *kafka.Reader
	notificationService *service.NotificationService
	log                 *logger.Logger
}

func NewUserWelcomeConsumer(
	brokers []string,
	topic string,
	groupID string,
	notificationService *service.NotificationService,
	log *logger.Logger,
) *UserWelcomeConsumer {
	log.Debug("UserWelcomeConsumer: создание",
		"brokers", brokers,
		"topic", topic,
		"group_id", groupID,
	)

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:           brokers,
		Topic:             topic,
		GroupID:           groupID,
		MinBytes:          1,
		MaxBytes:          10e6,
		MaxWait:           500 * time.Millisecond,
		CommitInterval:    5 * time.Second,
		SessionTimeout:    30 * time.Second,
		HeartbeatInterval: 3 * time.Second,
	})

	return &UserWelcomeConsumer{
		reader:              reader,
		notificationService: notificationService,
		log:                 log,
	}
}

func (c *UserWelcomeConsumer) Start(ctx context.Context) {
	c.log.Info("UserWelcomeConsumer: запуск",
		"topic", c.reader.Config().Topic,
	)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("UserWelcomeConsumer: остановка")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				c.log.Error("UserWelcomeConsumer: ошибка чтения сообщения",
					"error", err,
				)
				time.Sleep(time.Second)
				continue
			}

			c.processMessage(ctx, msg)

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.log.Error("UserWelcomeConsumer: ошибка коммита",
					"error", err,
				)
			}
		}
	}
}

func (c *UserWelcomeConsumer) processMessage(ctx context.Context, msg kafka.Message) {
	c.log.Debug("UserWelcomeConsumer: получено сообщение",
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
		"size", len(msg.Value),
	)

	var base BaseEvent
	if err := json.Unmarshal(msg.Value, &base); err != nil {
		c.log.Error("UserWelcomeConsumer: ошибка парсинга базового события",
			"error", err,
		)
		return
	}

	c.log.Debug("UserWelcomeConsumer: базовое событие",
		"event_id", base.EventID,
		"event_type", base.EventType,
	)

	if base.EventType != "user.welcome" {
		c.log.Debug("UserWelcomeConsumer: пропуск события другого типа",
			"event_type", base.EventType,
		)
		return
	}

	var data UserWelcomeData
	if err := json.Unmarshal(base.Data, &data); err != nil {
		c.log.Error("UserWelcomeConsumer: ошибка парсинга данных",
			"error", err,
		)
		return
	}

	c.log.Debug("UserWelcomeConsumer: данные события",
		"user_id", data.UserID,
		"email", data.Email,
		"first_name", data.FirstName,
	)

	// Создаем in-app уведомление
	notificationData := models.JSONMap{
		"welcome_type": "new_user",
	}

	title := "Добро пожаловать в Trip Builder!"
	body := "Спасибо за регистрацию! Начните планировать свои путешествия прямо сейчас."

	_, err := c.notificationService.CreateNotification(
		ctx,
		data.UserID,
		models.NotificationTypeWelcome,
		title,
		body,
		notificationData,
	)
	if err != nil {
		c.log.Error("UserWelcomeConsumer: ошибка создания in-app уведомления",
			"error", err,
			"user_id", data.UserID,
		)
		// Продолжаем, email важнее
	}

	// Отправляем приветственное письмо
	c.notificationService.SendWelcomeEmail(
		ctx,
		data.UserID,
		data.Email,
		data.FirstName,
		data.LastName,
	)

	c.log.Info("UserWelcomeConsumer: уведомления отправлены",
		"user_id", data.UserID,
		"email", data.Email,
	)
}

func (c *UserWelcomeConsumer) Close() error {
	c.log.Debug("UserWelcomeConsumer: закрытие")
	return c.reader.Close()
}
