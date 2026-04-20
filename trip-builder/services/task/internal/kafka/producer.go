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

// PublishTaskAssigned публикует событие о назначении задачи
func (p *TaskProducer) PublishTaskAssigned(ctx context.Context, taskID, eventID, assignedTo uuid.UUID, taskTitle, taskDescription, eventTitle, assignedEmail, assignedName, priority string, dueDate time.Time, assignedBy uuid.UUID, assignedByName string) error {
	p.log.Debug("Подготовка события task.assigned",
		"task_id", taskID,
		"assigned_to", assignedTo,
	)

	kafkaEvent := kafka.Event{
		EventID:   uuid.New().String(),
		EventType: "task.assigned",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"task_id":          taskID.String(),
			"task_title":       taskTitle,
			"task_description": taskDescription,
			"event_id":         eventID.String(),
			"event_title":      eventTitle,
			"assigned_to":      assignedTo.String(),
			"assigned_email":   assignedEmail,
			"assigned_name":    assignedName,
			"priority":         priority,
			"due_date":         dueDate,
			"assigned_by":      assignedBy.String(),
			"assigned_by_name": assignedByName,
		},
	}

	err := p.producer.PublishTo(ctx, "task.assigned", kafkaEvent)
	if err != nil {
		p.log.Error("Ошибка публикации task.assigned",
			"error", err,
			"task_id", taskID,
		)
		return err
	}

	p.log.Info("Событие task.assigned опубликовано",
		"task_id", taskID,
		"assigned_to", assignedTo,
	)
	return nil
}
