package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
	log    *logger.Logger
}

type Event struct {
	EventID   string      `json:"event_id"`
	EventType string      `json:"event_type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

func NewProducer(brokers []string, topic string, log *logger.Logger) *Producer {
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
		// Добавляем ретраи
		MaxAttempts: 10,
		BatchTimeout: 100 * time.Millisecond,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		RequiredAcks: kafka.RequireOne, // Ждём подтверждения от лидера
	}

	return &Producer{
		writer: writer,
		log:    log,
	}
}

func (p *Producer) Publish(ctx context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		p.log.Error("Ошибка маршалинга события", "error", err, "event_type", event.EventType)
		return err
	}

	msg := kafka.Message{
		Key:   []byte(event.EventID),
		Value: data,
		Time:  event.Timestamp,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
		},
	}

	// Добавляем свои ретраи с экспоненциальной задержкой
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		err = p.writer.WriteMessages(ctx, msg)
		if err == nil {
			p.log.Debug("Событие отправлено в Kafka", "event_type", event.EventType, "event_id", event.EventID)
			return nil
		}

		// Если ошибка не связана с лидером, выходим сразу
		if !isLeaderNotAvailableError(err) {
			break
		}

		// Экспоненциальная задержка
		waitTime := time.Duration(1<<attempt) * 500 * time.Millisecond
		p.log.Warn("Kafka лидер не доступен, повтор через", "attempt", attempt+1, "wait", waitTime)
		time.Sleep(waitTime)
	}

	p.log.Error("Ошибка отправки в Kafka", "error", err, "event_type", event.EventType)
	return err
}

func isLeaderNotAvailableError(err error) bool {
	var kafkaError kafka.Error
	if errors.As(err, &kafkaError) {
		return kafkaError == kafka.LeaderNotAvailable
	}
	return false
}

func (p *Producer) Close() error {
	return p.writer.Close()
}