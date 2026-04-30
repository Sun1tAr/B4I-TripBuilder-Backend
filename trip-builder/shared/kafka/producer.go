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
	// Добавляем возможность создавать продюсеры для разных топиков
	brokers []string
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
		MaxAttempts:            10,
		BatchTimeout:           100 * time.Millisecond,
		ReadTimeout:            10 * time.Second,
		WriteTimeout:           10 * time.Second,
		RequiredAcks:           kafka.RequireOne,
	}

	return &Producer{
		writer:  writer,
		log:     log,
		brokers: brokers, // сохраняем для создания копий
	}
}

// Publish публикует событие в текущий топик (существующий метод, не меняем)
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

	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		err = p.writer.WriteMessages(ctx, msg)
		if err == nil {
			p.log.Debug("Событие отправлено в Kafka",
				"topic", p.writer.Topic,
				"event_type", event.EventType,
				"event_id", event.EventID,
			)
			return nil
		}

		if !isLeaderNotAvailableError(err) {
			break
		}

		waitTime := time.Duration(1<<attempt) * 500 * time.Millisecond
		p.log.Warn("Kafka лидер не доступен, повтор через",
			"attempt", attempt+1,
			"wait", waitTime,
			"topic", p.writer.Topic,
		)
		time.Sleep(waitTime)
	}

	p.log.Error("Ошибка отправки в Kafka",
		"error", err,
		"topic", p.writer.Topic,
		"event_type", event.EventType,
	)
	return err
}

// WithTopic создаёт новый продюсер для другого топика (НОВЫЙ МЕТОД)
// Не изменяет исходный продюсер, а создаёт новый с тем же набором брокеров
func (p *Producer) WithTopic(topic string) *Producer {
	p.log.Debug("Создание продюсера для нового топика",
		"from_topic", p.writer.Topic,
		"to_topic", topic,
	)

	// Просто вызываем NewProducer с теми же брокерами и новым топиком
	return NewProducer(p.brokers, topic, p.log)
}

// PublishTo публикует событие в указанный топик (НОВЫЙ МЕТОД)
// Удобный метод для публикации в другой топик без создания отдельного продюсера
func (p *Producer) PublishTo(ctx context.Context, topic string, event Event) error {
	p.log.Debug("Публикация в другой топик",
		"target_topic", topic,
		"current_topic", p.writer.Topic,
		"event_type", event.EventType,
	)

	// Создаём временный продюсер
	tmpProducer := NewProducer(p.brokers, topic, p.log)
	defer tmpProducer.Close() // важно закрыть, чтобы не было утечки

	return tmpProducer.Publish(ctx, event)
}

// Close закрывает продюсер
func (p *Producer) Close() error {
	p.log.Debug("Закрытие Kafka продюсера", "topic", p.writer.Topic)
	return p.writer.Close()
}

func isLeaderNotAvailableError(err error) bool {
	var kafkaError kafka.Error
	if errors.As(err, &kafkaError) {
		return kafkaError == kafka.LeaderNotAvailable
	}
	return false
}
