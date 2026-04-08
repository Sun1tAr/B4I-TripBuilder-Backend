package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	ServiceName string `envconfig:"SERVICE_NAME" required:"true"`
	HTTPPort    string `envconfig:"HTTP_PORT" default:"8088"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`
	LogFormat   string `envconfig:"LOG_FORMAT" default:"console"`

	// Elasticsearch
	ElasticsearchAddrs []string `envconfig:"ELASTICSEARCH_ADDRS" default:"http://localhost:9200"`
	ElasticsearchUser  string   `envconfig:"ELASTICSEARCH_USER" default:""`
	ElasticsearchPass  string   `envconfig:"ELASTICSEARCH_PASSWORD" default:""`

	// Redis (для резерва)
	Redis config.Redis

	// Kafka
	KafkaBrokers []string `envconfig:"KAFKA_BROKERS" default:"localhost:9092"`

	// Kafka топики
	RouteCreatedTopic string `envconfig:"KAFKA_TOPIC_ROUTE_CREATED" default:"route.created"`
	RouteUpdatedTopic string `envconfig:"KAFKA_TOPIC_ROUTE_UPDATED" default:"route.updated"`
	RouteDeletedTopic string `envconfig:"KAFKA_TOPIC_ROUTE_DELETED" default:"route.deleted"`

	EventCreatedTopic string `envconfig:"KAFKA_TOPIC_EVENT_CREATED" default:"event.created"`
	EventUpdatedTopic string `envconfig:"KAFKA_TOPIC_EVENT_UPDATED" default:"event.updated"`
	EventDeletedTopic string `envconfig:"KAFKA_TOPIC_EVENT_DELETED" default:"event.deleted"`

	UserCreatedTopic string `envconfig:"KAFKA_TOPIC_USER_CREATED" default:"user.created"`
	UserUpdatedTopic string `envconfig:"KAFKA_TOPIC_USER_UPDATED" default:"user.updated"`

	// URLs других сервисов для реиндексации
	RouteServiceURL string `envconfig:"ROUTE_SERVICE_URL" required:"true"`
	EventServiceURL string `envconfig:"EVENT_SERVICE_URL" required:"true"`
	UserServiceURL  string `envconfig:"USER_SERVICE_URL" required:"true"`

	// ES health check
	ESHealthCheckInterval time.Duration `envconfig:"ES_HEALTH_CHECK_INTERVAL" default:"1h"`

	// Retry при падении ES
	ESMaxRetries int           `envconfig:"ES_MAX_RETRIES" default:"3"`
	ESRetryDelay time.Duration `envconfig:"ES_RETRY_DELAY" default:"100ms"`

	// Пагинация
	MaxResultWindow int `envconfig:"MAX_RESULT_WINDOW" default:"10000"`

	SearchMode        string        `envconfig:"SEARCH_MODE" default:"proxy"`       // full, proxy
	HTTPClientTimeout time.Duration `envconfig:"HTTP_CLIENT_TIMEOUT" default:"30s"` // Таймауты HTTP клиентов
}

func (c *Config) GetLogLevel() slog.Level {
	switch strings.ToLower(c.LogLevel) {
	case "trace":
		return logger.LevelTrace
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Load() (*Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &cfg, nil
}

func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		slog.Error("Ошибка загрузки конфигурации", "error", err)
		os.Exit(1)
	}
	return cfg
}
