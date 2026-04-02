package kafka

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type TaskProducer struct {
	producer *kafka.Producer
	log      *logger.Logger
}

func NewTaskProducer(brokers []string, log *logger.Logger) *TaskProducer {
	log.Debug("Создание TaskProducer", "brokers", brokers)

	baseProducer := kafka.NewProducer(brokers, "task", log)

	return &TaskProducer{
		producer: baseProducer,
		log:      log,
	}
}

// PublishTaskCompleted публикует событие о выполнении задачи
func (p *TaskProducer) PublishTaskCompleted(ctx context.Context, taskID, eventID, userID uuid.UUID, title string) error {
	p.log.Debug("Подготовка события task.completed",
		"task_id", taskID,
		"event_id", eventID,
		"user_id", userID,
	)

	kafkaEvent := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "task.completed",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"task_id":      taskID.String(),
			"event_id":     eventID.String(),
			"user_id":      userID.String(),
			"title":        title,
			"completed_at": time.Now(),
		},
	}

	err := p.producer.PublishTo(ctx, "task.completed", kafkaEvent)
	if err != nil {
		p.log.Error("Ошибка публикации task.completed",
			"error", err,
			"task_id", taskID,
		)
		return err
	}

	p.log.Info("Событие task.completed опубликовано", "task_id", taskID, "event_id", eventID)
	return nil
}

// Close закрывает продюсер
func (p *TaskProducer) Close() error {
	p.log.Debug("Закрытие TaskProducer")
	return p.producer.Close()
}
