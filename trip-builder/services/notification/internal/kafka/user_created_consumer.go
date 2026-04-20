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

type UserCreatedConsumer struct {
	reader              *kafka.Reader
	notificationService *service.NotificationService
	log                 *logger.Logger
}

func NewUserCreatedConsumer(
	brokers []string,
	topic string,
	groupID string,
	notificationService *service.NotificationService,
	log *logger.Logger,
) *UserCreatedConsumer {
	log.Debug("UserCreatedConsumer: создание",
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

	return &UserCreatedConsumer{
		reader:              reader,
		notificationService: notificationService,
		log:                 log,
	}
}

func (c *UserCreatedConsumer) Start(ctx context.Context) {
	c.log.Info("UserCreatedConsumer: запуск",
		"topic", c.reader.Config().Topic,
	)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("UserCreatedConsumer: остановка")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				c.log.Error("UserCreatedConsumer: ошибка чтения сообщения", "error", err)
				time.Sleep(time.Second)
				continue
			}

			c.processMessage(ctx, msg)

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.log.Error("UserCreatedConsumer: ошибка коммита", "error", err)
			}
		}
	}
}

func (c *UserCreatedConsumer) processMessage(ctx context.Context, msg kafka.Message) {
	c.log.Debug("UserCreatedConsumer: получено сообщение",
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
	)

	var base BaseEvent
	if err := json.Unmarshal(msg.Value, &base); err != nil {
		c.log.Error("UserCreatedConsumer: ошибка парсинга базового события", "error", err)
		return
	}

	if base.EventType != "user.created" {
		c.log.Debug("UserCreatedConsumer: пропуск события другого типа",
			"event_type", base.EventType,
		)
		return
	}

	var data UserCreatedData
	if err := json.Unmarshal(base.Data, &data); err != nil {
		c.log.Error("UserCreatedConsumer: ошибка парсинга данных", "error", err)
		return
	}

	c.log.Debug("UserCreatedConsumer: данные события",
		"user_id", data.UserID,
		"email", data.Email,
		"first_name", data.FirstName,
		"has_verification_token", data.VerificationToken != "",
	)

	// 1. Создаем in-app уведомление
	notificationData := models.JSONMap{
		"welcome_type": "new_user",
	}

	title := "Добро пожаловать в Trip Builder!"
	body := "Пожалуйста, подтвердите ваш email, перейдя по ссылке в письме."

	_, err := c.notificationService.CreateNotification(
		ctx,
		data.UserID,
		models.NotificationTypeWelcome,
		title,
		body,
		notificationData,
	)
	if err != nil {
		c.log.Error("UserCreatedConsumer: ошибка создания in-app уведомления",
			"error", err,
			"user_id", data.UserID,
		)
	}

	// 2. Отправляем письмо с подтверждением email
	c.notificationService.SendVerificationEmail(
		ctx,
		data.UserID,
		data.Email,
		data.FirstName,
		data.LastName,
		data.VerificationToken,
	)

	c.log.Info("UserCreatedConsumer: уведомления отправлены",
		"user_id", data.UserID,
		"email", data.Email,
	)
}

func (c *UserCreatedConsumer) Close() error {
	c.log.Debug("UserCreatedConsumer: закрытие")
	return c.reader.Close()
}
