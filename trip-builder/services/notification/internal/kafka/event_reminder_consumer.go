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

type EventReminderConsumer struct {
	reader              *kafka.Reader
	notificationService *service.NotificationService
	log                 *logger.Logger
}

func NewEventReminderConsumer(
	brokers []string,
	topic string,
	groupID string,
	notificationService *service.NotificationService,
	log *logger.Logger,
) *EventReminderConsumer {
	log.Debug("EventReminderConsumer: создание",
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

	return &EventReminderConsumer{
		reader:              reader,
		notificationService: notificationService,
		log:                 log,
	}
}

func (c *EventReminderConsumer) Start(ctx context.Context) {
	c.log.Info("EventReminderConsumer: запуск",
		"topic", c.reader.Config().Topic,
	)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("EventReminderConsumer: остановка")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				c.log.Error("EventReminderConsumer: ошибка чтения сообщения", "error", err)
				time.Sleep(time.Second)
				continue
			}

			c.processMessage(ctx, msg)

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.log.Error("EventReminderConsumer: ошибка коммита", "error", err)
			}
		}
	}
}

func (c *EventReminderConsumer) processMessage(ctx context.Context, msg kafka.Message) {
	c.log.Debug("EventReminderConsumer: получено сообщение",
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
	)

	var base BaseEvent
	if err := json.Unmarshal(msg.Value, &base); err != nil {
		c.log.Error("EventReminderConsumer: ошибка парсинга базового события", "error", err)
		return
	}

	if base.EventType != "event.reminder" {
		c.log.Debug("EventReminderConsumer: пропуск события другого типа",
			"event_type", base.EventType,
		)
		return
	}

	var data EventReminderData
	if err := json.Unmarshal(base.Data, &data); err != nil {
		c.log.Error("EventReminderConsumer: ошибка парсинга данных", "error", err)
		return
	}

	c.log.Debug("EventReminderConsumer: данные события",
		"event_id", data.EventID,
		"event_title", data.EventTitle,
		"days_left", data.DaysLeft,
		"participants_count", len(data.Participants),
	)

	if len(data.Participants) == 0 {
		c.log.Debug("EventReminderConsumer: нет участников, пропускаем")
		return
	}

	// Собираем ID пользователей для массового создания in-app уведомлений
	userIDs := make([]uuid.UUID, 0, len(data.Participants))
	for _, p := range data.Participants {
		userIDs = append(userIDs, p.UserID)
	}

	// Формируем текст в зависимости от оставшихся дней
	var notificationBody string
	if data.DaysLeft == 0 {
		notificationBody = fmt.Sprintf("Поездка \"%s\" начинается сегодня!", data.EventTitle)
	} else if data.DaysLeft == 1 {
		notificationBody = fmt.Sprintf("Поездка \"%s\" начинается завтра!", data.EventTitle)
	} else {
		notificationBody = fmt.Sprintf("До начала поездки \"%s\" осталось %d дней",
			data.EventTitle, data.DaysLeft)
	}

	notificationTitle := models.GetTitleByType(models.NotificationTypeEventReminder)
	notificationData := models.JSONMap{
		"event_id":    data.EventID.String(),
		"event_title": data.EventTitle,
		"start_date":  data.StartDate,
		"days_left":   data.DaysLeft,
	}

	if err := c.notificationService.CreateNotificationsBatch(
		ctx,
		userIDs,
		models.NotificationTypeEventReminder,
		notificationTitle,
		notificationBody,
		notificationData,
	); err != nil {
		c.log.Error("EventReminderConsumer: ошибка создания in-app уведомлений",
			"error", err,
			"event_id", data.EventID,
		)
	} else {
		c.log.Info("EventReminderConsumer: in-app уведомления созданы",
			"event_id", data.EventID,
			"count", len(userIDs),
		)
	}

	// Отправляем email каждому участнику
	startDateStr := data.StartDate.Format("02.01.2006")

	for _, participant := range data.Participants {
		c.notificationService.SendEventReminderEmail(
			ctx,
			participant.UserID,
			participant.Email,
			participant.FirstName,
			data.EventTitle,
			startDateStr,
			data.DaysLeft,
			data.EventID,
		)

		c.log.Debug("EventReminderConsumer: email отправлен в очередь",
			"user_id", participant.UserID,
			"email", participant.Email,
		)
	}

	c.log.Info("EventReminderConsumer: обработка завершена",
		"event_id", data.EventID,
		"participants_count", len(data.Participants),
	)
}

func (c *EventReminderConsumer) Close() error {
	c.log.Debug("EventReminderConsumer: закрытие")
	return c.reader.Close()
}
