package retry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/cenkalti/backoff/v4"
	"github.com/segmentio/kafka-go"
)

type RetryConsumer struct {
	reader     *kafka.Reader
	producer   *kafka.Writer
	config     RetryConfig
	log        *logger.Logger
	httpClient *http.Client
}

func NewRetryConsumer(cfg RetryConfig, log *logger.Logger) *RetryConsumer {
	return &RetryConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  cfg.Brokers,
			Topic:    cfg.RetryTopic,
			GroupID:  "retry-consumer",
			MinBytes: 10e3,
			MaxBytes: 10e6,
			MaxWait:  1 * time.Second,
		}),
		producer: &kafka.Writer{
			Addr:  kafka.TCP(cfg.Brokers...),
			Topic: cfg.DLQTopic,
		},
		config: cfg,
		log:    log,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *RetryConsumer) Start(ctx context.Context) {
	c.log.Info("Запуск RetryConsumer для топика", "topic", c.config.RetryTopic)
	for {
		select {
		case <-ctx.Done():
			c.log.Info("Остановка RetryConsumer")
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				c.log.Error("Ошибка чтения из retry топика", "error", err)
				continue
			}

			c.processMessage(msg)
		}
	}
}

func (c *RetryConsumer) processMessage(msg kafka.Message) {
	ctx := context.Background()
	// Получаем количество попыток из заголовков
	retryCount := 0
	for _, h := range msg.Headers {
		if h.Key == "retry-count" {
			fmt.Sscanf(string(h.Value), "%d", &retryCount)
			break
		}
	}

	var req FailedRequest
	if err := json.Unmarshal(msg.Value, &req); err != nil {
		c.log.Error("Ошибка декодирования запроса", "error", err)
		c.sendToDLQ(msg, "decode_error")
		c.reader.CommitMessages(ctx, msg)
		return
	}

	// Пробуем выполнить с экспоненциальным backoff
	err := c.retryWithBackoff(ctx, req)
	if err != nil {
		if retryCount >= c.config.MaxRetries {
			c.log.Error("Превышено максимальное количество попыток", "url", req.URL)
			c.sendToDLQ(msg, "max_retries_exceeded")
		} else {
			// Отправляем на повтор с увеличенным счетчиком
			retryCount++
			req.Attempts = retryCount
			req.LastError = err.Error()
			req.LastAttempt = time.Now()

			data, _ := json.Marshal(req)
			c.producer.WriteMessages(ctx, kafka.Message{
				Value: data,
				Headers: []kafka.Header{
					{Key: "retry-count", Value: []byte(fmt.Sprintf("%d", retryCount))},
				},
			})
		}
	}

	c.reader.CommitMessages(ctx, msg)
}

func (c *RetryConsumer) retryWithBackoff(ctx context.Context, req FailedRequest) error {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = c.config.InitialBackoff
	b.MaxInterval = c.config.MaxBackoff
	b.MaxElapsedTime = 0

	operation := func() error {
		httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, nil)
		if err != nil {
			return err
		}
		for k, v := range req.Headers {
			httpReq.Header.Set(k, v)
		}
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("temporary error: %d", resp.StatusCode)
		}
		if resp.StatusCode >= 400 {
			// Постоянная ошибка клиента — не повторяем
			return nil
		}
		return nil
	}

	return backoff.Retry(operation, backoff.WithMaxRetries(b, uint64(c.config.MaxRetries-req.Attempts)))
}

func (c *RetryConsumer) sendToDLQ(msg kafka.Message, reason string) {
	dlqMsg := kafka.Message{
		Value: msg.Value,
		Headers: append(msg.Headers,
			kafka.Header{Key: "dlq-reason", Value: []byte(reason)},
			kafka.Header{Key: "dlq-timestamp", Value: []byte(time.Now().Format(time.RFC3339))},
		),
	}
	if err := c.producer.WriteMessages(context.Background(), dlqMsg); err != nil {
		c.log.Error("Ошибка отправки в DLQ", "error", err)
	}
}