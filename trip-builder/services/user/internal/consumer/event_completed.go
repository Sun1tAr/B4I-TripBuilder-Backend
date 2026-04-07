package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/segmentio/kafka-go"
)

type EventCompletedConsumer struct {
	reader       *kafka.Reader
	statsService *service.StatsService
	log          *logger.Logger
}

func NewEventCompletedConsumer(brokers []string, topic string, groupID string, statsService *service.StatsService, log *logger.Logger) *EventCompletedConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3,
		MaxBytes: 10e6,
		MaxWait:  1 * time.Second,
	})
	return &EventCompletedConsumer{
		reader:       reader,
		statsService: statsService,
		log:          log,
	}
}

func (c *EventCompletedConsumer) Start(ctx context.Context) {
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

func (c *EventCompletedConsumer) processMessage(ctx context.Context, msg kafka.Message) {

	var base BaseEvent
	if err := json.Unmarshal(msg.Value, &base); err != nil {
		c.log.Error("Ошибка парсинга базового события", "error", err)
		return
	}

	var data EventCompletedData
	if err := json.Unmarshal(base.Data, &data); err != nil {
		c.log.Error("Ошибка парсинга данных event.completed", "error", err)
		return
	}

	c.log.Debug("Обработка event.completed",
		"event_id", data.EventID,
		"participants", len(data.Participants),
		"countries", data.CountriesVisited,
	)

	// Обновляем статистику для каждого участника
	for _, participantID := range data.Participants {
		if err := c.statsService.IncrementEventsDone(ctx, participantID); err != nil {
			c.log.Error("Ошибка инкремента events_done",
				"error", err,
				"user_id", participantID,
			)
			// продолжаем для остальных участников
		}

		if len(data.CountriesVisited) > 0 {
			if err := c.statsService.IncrementCountriesVisited(ctx, participantID, len(data.CountriesVisited)); err != nil {
				c.log.Error("Ошибка инкремента countries_visited",
					"error", err,
					"user_id", participantID,
				)
			}
		}
	}

	c.log.Info("Статистика участников обновлена", "event_id", data.EventID)
}

func (c *EventCompletedConsumer) Close() error {
	return c.reader.Close()
}
