package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/service"
	authdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/auth"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/segmentio/kafka-go"
)

type PasswordResetConsumer struct {
	reader              *kafka.Reader
	notificationService *service.NotificationService
	log                 *logger.Logger
}

func NewPasswordResetConsumer(
	brokers []string, topic, groupID string,
	notificationService *service.NotificationService,
	log *logger.Logger,
) *PasswordResetConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers, Topic: topic, GroupID: groupID,
		MinBytes: 1, MaxBytes: 10e6,
		MaxWait:           500 * time.Millisecond,
		CommitInterval:    5 * time.Second,
		SessionTimeout:    30 * time.Second,
		HeartbeatInterval: 3 * time.Second,
	})
	return &PasswordResetConsumer{reader: reader, notificationService: notificationService, log: log}
}

func (c *PasswordResetConsumer) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			c.processMessage(ctx, msg)
			c.reader.CommitMessages(ctx, msg)
		}
	}
}

func (c *PasswordResetConsumer) processMessage(ctx context.Context, msg kafka.Message) {
	var base BaseEvent
	if err := json.Unmarshal(msg.Value, &base); err != nil {
		return
	}

	var data authdto.PasswordResetData
	if err := json.Unmarshal(base.Data, &data); err != nil {
		return
	}

	c.log.Info("PasswordResetConsumer: отправка письма", "email", data.Email)

	// Создаем in-app уведомление
	c.notificationService.CreateNotification(ctx, data.UserID,
		"password_reset",
		"Сброс пароля",
		"На вашу почту отправлена ссылка для сброса пароля",
		models.JSONMap{"reset_token": data.ResetToken},
	)

	// Отправляем email
	c.notificationService.SendPasswordResetEmail(ctx, data.UserID, data.Email, data.FirstName, data.ResetToken)
}
