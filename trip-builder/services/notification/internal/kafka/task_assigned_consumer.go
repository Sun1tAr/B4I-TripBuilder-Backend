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
	"github.com/segmentio/kafka-go"
)

type TaskAssignedConsumer struct {
	reader              *kafka.Reader
	notificationService *service.NotificationService
	log                 *logger.Logger
}

func NewTaskAssignedConsumer(
	brokers []string,
	topic string,
	groupID string,
	notificationService *service.NotificationService,
	log *logger.Logger,
) *TaskAssignedConsumer {
	log.Debug("TaskAssignedConsumer: создание",
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

	return &TaskAssignedConsumer{
		reader:              reader,
		notificationService: notificationService,
		log:                 log,
	}
}

func (c *TaskAssignedConsumer) Start(ctx context.Context) {
	c.log.Info("TaskAssignedConsumer: запуск",
		"topic", c.reader.Config().Topic,
	)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("TaskAssignedConsumer: остановка")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				c.log.Error("TaskAssignedConsumer: ошибка чтения сообщения", "error", err)
				time.Sleep(time.Second)
				continue
			}

			c.processMessage(ctx, msg)

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.log.Error("TaskAssignedConsumer: ошибка коммита", "error", err)
			}
		}
	}
}

func (c *TaskAssignedConsumer) processMessage(ctx context.Context, msg kafka.Message) {
	c.log.Debug("TaskAssignedConsumer: получено сообщение",
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
	)

	var base BaseEvent
	if err := json.Unmarshal(msg.Value, &base); err != nil {
		c.log.Error("TaskAssignedConsumer: ошибка парсинга базового события", "error", err)
		return
	}

	if base.EventType != "task.assigned" {
		c.log.Debug("TaskAssignedConsumer: пропуск события другого типа",
			"event_type", base.EventType,
		)
		return
	}

	var data TaskAssignedData
	if err := json.Unmarshal(base.Data, &data); err != nil {
		c.log.Error("TaskAssignedConsumer: ошибка парсинга данных", "error", err)
		return
	}

	c.log.Debug("TaskAssignedConsumer: данные события",
		"task_id", data.TaskID,
		"task_title", data.TaskTitle,
		"assigned_to", data.AssignedTo,
	)

	// Создаем in-app уведомление
	notificationTitle := models.GetTitleByType(models.NotificationTypeTaskAssigned)
	notificationBody := fmt.Sprintf("Вам назначена задача \"%s\" в поездке \"%s\"",
		data.TaskTitle, data.EventTitle)

	notificationData := models.JSONMap{
		"task_id":     data.TaskID.String(),
		"event_id":    data.EventID.String(),
		"task_title":  data.TaskTitle,
		"event_title": data.EventTitle,
		"priority":    data.Priority,
		"due_date":    data.DueDate,
		"assigned_by": data.AssignedBy.String(),
	}

	_, err := c.notificationService.CreateNotification(
		ctx,
		data.AssignedTo,
		models.NotificationTypeTaskAssigned,
		notificationTitle,
		notificationBody,
		notificationData,
	)
	if err != nil {
		c.log.Error("TaskAssignedConsumer: ошибка создания in-app уведомления",
			"error", err,
			"user_id", data.AssignedTo,
		)
	} else {
		c.log.Debug("TaskAssignedConsumer: in-app уведомление создано",
			"user_id", data.AssignedTo,
		)
	}

	// Отправляем email
	dueDateStr := ""
	if !data.DueDate.IsZero() {
		dueDateStr = data.DueDate.Format("02.01.2006 15:04")
	}

	c.notificationService.SendTaskAssignedEmail(
		ctx,
		data.AssignedTo,
		data.AssignedEmail,
		data.AssignedName,
		data.TaskTitle,
		data.TaskDescription,
		data.EventTitle,
		dueDateStr,
		data.Priority,
		data.TaskID,
		data.EventID,
	)

	c.log.Info("TaskAssignedConsumer: обработка завершена",
		"task_id", data.TaskID,
		"assigned_to", data.AssignedTo,
	)
}

func (c *TaskAssignedConsumer) Close() error {
	c.log.Debug("TaskAssignedConsumer: закрытие")
	return c.reader.Close()
}
