package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

type ChatMessageConsumer struct {
	reader              *kafka.Reader
	notificationService *service.NotificationService
	log                 *logger.Logger
}

func NewChatMessageConsumer(
	brokers []string,
	topic string,
	groupID string,
	notificationService *service.NotificationService,
	log *logger.Logger,
) *ChatMessageConsumer {
	log.Debug("ChatMessageConsumer: создание",
		"brokers", brokers,
		"topic", topic,
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

	return &ChatMessageConsumer{
		reader:              reader,
		notificationService: notificationService,
		log:                 log,
	}
}

func (c *ChatMessageConsumer) Start(ctx context.Context) {
	c.log.Info("ChatMessageConsumer: запуск",
		"topic", c.reader.Config().Topic,
	)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("ChatMessageConsumer: остановка")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				c.log.Error("ChatMessageConsumer: ошибка чтения сообщения", "error", err)
				time.Sleep(time.Second)
				continue
			}

			c.processMessage(ctx, msg)

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.log.Error("ChatMessageConsumer: ошибка коммита", "error", err)
			}
		}
	}
}

func (c *ChatMessageConsumer) processMessage(ctx context.Context, msg kafka.Message) {
	c.log.Debug("ChatMessageConsumer: получено сообщение",
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
	)

	var base BaseEvent
	if err := json.Unmarshal(msg.Value, &base); err != nil {
		c.log.Error("ChatMessageConsumer: ошибка парсинга базового события", "error", err)
		return
	}

	if base.EventType != "chat.message" {
		c.log.Debug("ChatMessageConsumer: пропуск события другого типа",
			"event_type", base.EventType,
		)
		return
	}

	var data ChatMessageData
	if err := json.Unmarshal(base.Data, &data); err != nil {
		c.log.Error("ChatMessageConsumer: ошибка парсинга данных", "error", err)
		return
	}

	c.log.Debug("ChatMessageConsumer: данные события",
		"message_id", data.MessageID,
		"event_id", data.EventID,
		"sender_id", data.SenderID,
		"offline_recipients", len(data.OfflineRecipients),
	)

	if len(data.OfflineRecipients) == 0 {
		c.log.Debug("ChatMessageConsumer: нет офлайн-получателей, пропускаем")
		return
	}

	// Собираем ID пользователей для массового создания in-app уведомлений
	userIDs := make([]uuid.UUID, 0, len(data.OfflineRecipients))
	for _, recipient := range data.OfflineRecipients {
		userIDs = append(userIDs, recipient.UserID)
	}

	// Создаем in-app уведомления для офлайн-участников
	notificationTitle := models.GetTitleByType(models.NotificationTypeNewMessage)
	notificationBody := fmt.Sprintf("%s: %s", data.SenderName, truncateForNotification(data.MessageText, 100))
	notificationData := models.JSONMap{
		"event_id":    data.EventID.String(),
		"event_title": data.EventTitle,
		"message_id":  data.MessageID.String(),
		"sender_id":   data.SenderID.String(),
		"sender_name": data.SenderName,
	}

	if err := c.notificationService.CreateNotificationsBatch(
		ctx,
		userIDs,
		models.NotificationTypeNewMessage,
		notificationTitle,
		notificationBody,
		notificationData,
	); err != nil {
		c.log.Error("ChatMessageConsumer: ошибка создания in-app уведомлений",
			"error", err,
			"event_id", data.EventID,
		)
	} else {
		c.log.Info("ChatMessageConsumer: in-app уведомления созданы",
			"event_id", data.EventID,
			"count", len(userIDs),
		)
	}

	// Отправляем email каждому офлайн-получателю
	for _, recipient := range data.OfflineRecipients {
		c.notificationService.SendChatMessageEmail(
			ctx,
			recipient.UserID,
			recipient.Email,
			recipient.FirstName,
			data.EventTitle,
			data.SenderName,
			data.MessageText,
			data.EventID,
		)

		c.log.Debug("ChatMessageConsumer: email отправлен в очередь",
			"user_id", recipient.UserID,
			"email", recipient.Email,
		)
	}

	c.log.Info("ChatMessageConsumer: обработка завершена",
		"message_id", data.MessageID,
		"offline_count", len(data.OfflineRecipients),
	)
}

func (c *ChatMessageConsumer) Close() error {
	c.log.Debug("ChatMessageConsumer: закрытие")
	return c.reader.Close()
}

func truncateForNotification(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
