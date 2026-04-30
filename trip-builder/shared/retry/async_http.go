package retry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/segmentio/kafka-go"
)

type RetryConfig struct {
	Brokers        []string
	RetryTopic     string
	DLQTopic       string
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

type FailedRequest struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body"`
	Attempts    int               `json:"attempts"`
	LastError   string            `json:"last_error"`
	LastAttempt time.Time         `json:"last_attempt"`
}

type AsyncHTTPClient struct {
	producer *kafka.Writer
	config   RetryConfig
	log      *logger.Logger
}

func NewAsyncHTTPClient(cfg RetryConfig, log *logger.Logger) *AsyncHTTPClient {
	return &AsyncHTTPClient{
		producer: &kafka.Writer{
			Addr:     kafka.TCP(cfg.Brokers...),
			Topic:    cfg.RetryTopic,
			Balancer: &kafka.LeastBytes{},
		},
		config: cfg,
		log:    log,
	}
}

// Enqueue помещает неудавшийся запрос в очередь повторных попыток
func (c *AsyncHTTPClient) Enqueue(ctx context.Context, req FailedRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	return c.producer.WriteMessages(ctx, kafka.Message{
		Value: data,
		Headers: []kafka.Header{
			{Key: "retry-count", Value: []byte(fmt.Sprintf("%d", req.Attempts))},
			{Key: "original-timestamp", Value: []byte(req.LastAttempt.Format(time.RFC3339))},
		},
	})
}

// ExecuteWithFallback оставлен как заглушка для совместимости
func (c *AsyncHTTPClient) ExecuteWithFallback(ctx context.Context, method, url string, headers map[string]string, body []byte) error {
	return nil
}