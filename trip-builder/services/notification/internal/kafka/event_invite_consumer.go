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

type EventInviteConsumer struct {
	reader              *kafka.Reader
	notificationService *service.NotificationService
	log                 *logger.Logger
}

func NewEventInviteConsumer(
	brokers []string,
	topic string,
	groupID string,
	notificationService *service.NotificationService,
	log *logger.Logger,
) *EventInviteConsumer {
	log.Debug("EventInviteConsumer: создание",
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

	return &EventInviteConsumer{
		reader:              reader,
		notificationService: notificationService,
		log:                 log,
	}
}

func (c *EventInviteConsumer) Start(ctx context.Context) {
	c.log.Info("EventInviteConsumer: запуск",
		"topic", c.reader.Config().Topic,
	)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("EventInviteConsumer: остановка")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				c.log.Error("EventInviteConsumer: ошибка чтения сообщения", "error", err)
				time.Sleep(time.Second)
				continue
			}

			c.processMessage(ctx, msg)

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.log.Error("EventInviteConsumer: ошибка коммита", "error", err)
			}
		}
	}
}

func (c *EventInviteConsumer) processMessage(ctx context.Context, msg kafka.Message) {
	c.log.Debug("EventInviteConsumer: получено сообщение",
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
	)

	var base BaseEvent
	if err := json.Unmarshal(msg.Value, &base); err != nil {
		c.log.Error("EventInviteConsumer: ошибка парсинга базового события", "error", err)
		return
	}

	if base.EventType != "event.invite" {
		c.log.Debug("EventInviteConsumer: пропуск события другого типа",
			"event_type", base.EventType,
		)
		return
	}

	var data EventInviteData
	if err := json.Unmarshal(base.Data, &data); err != nil {
		c.log.Error("EventInviteConsumer: ошибка парсинга данных", "error", err)
		return
	}

	c.log.Debug("EventInviteConsumer: данные события",
		"event_id", data.EventID,
		"event_title", data.EventTitle,
		"invited_users_count", len(data.InvitedUsers),
	)

	startDateStr := data.StartDate.Format("02.01.2006")

	// Собираем ID пользователей для массового создания in-app уведомлений
	userIDs := make([]uuid.UUID, 0, len(data.InvitedUsers))

	for _, user := range data.InvitedUsers {
		userIDs = append(userIDs, user.UserID)
	}

	// Создаем in-app уведомления для всех приглашенных
	notificationTitle := models.GetTitleByType(models.NotificationTypeEventInvite)
	notificationBody := fmt.Sprintf("%s приглашает вас в поездку \"%s\"", data.InviterName, data.EventTitle)
	notificationData := models.JSONMap{
		"event_id":    data.EventID.String(),
		"event_title": data.EventTitle,
		"inviter_id":  data.InviterID.String(),
		"start_date":  data.StartDate,
	}

	if err := c.notificationService.CreateNotificationsBatch(
		ctx,
		userIDs,
		models.NotificationTypeEventInvite,
		notificationTitle,
		notificationBody,
		notificationData,
	); err != nil {
		c.log.Error("EventInviteConsumer: ошибка создания in-app уведомлений",
			"error", err,
			"event_id", data.EventID,
		)
		// Продолжаем, email важнее
	} else {
		c.log.Info("EventInviteConsumer: in-app уведомления созданы",
			"event_id", data.EventID,
			"count", len(userIDs),
		)
	}

	// Отправляем email каждому приглашенному
	for _, user := range data.InvitedUsers {
		c.notificationService.SendEventInviteEmail(
			ctx,
			user.UserID,
			user.Email,
			user.FirstName,
			data.EventTitle,
			data.RouteTitle,
			data.InviterName,
			startDateStr,
			data.EventID,
		)

		c.log.Debug("EventInviteConsumer: email отправлен в очередь",
			"user_id", user.UserID,
			"email", user.Email,
		)
	}

	c.log.Info("EventInviteConsumer: обработка завершена",
		"event_id", data.EventID,
		"invited_count", len(data.InvitedUsers),
	)
}

func (c *EventInviteConsumer) Close() error {
	c.log.Debug("EventInviteConsumer: закрытие")
	return c.reader.Close()
}
