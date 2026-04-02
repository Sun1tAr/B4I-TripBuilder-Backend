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
	config.AppConfig

	// Kafka топики
	EventCreatedTopic   string `envconfig:"KAFKA_TOPIC_EVENT_CREATED" default:"event.created"`
	EventUpdatedTopic   string `envconfig:"KAFKA_TOPIC_EVENT_UPDATED" default:"event.updated"`
	EventCancelledTopic string `envconfig:"KAFKA_TOPIC_EVENT_CANCELLED" default:"event.cancelled"`
	TaskCompletedTopic  string `envconfig:"KAFKA_TOPIC_TASK_COMPLETED" default:"task.completed"`

	// URL других сервисов
	EventServiceURL string `envconfig:"EVENT_SERVICE_URL" required:"true"`

	// Настройки миграций
	MigrationEnabled bool `envconfig:"MIGRATION_ENABLED" default:"true"`

	// Таймауты HTTP клиента
	HTTPClientTimeout time.Duration `envconfig:"HTTP_CLIENT_TIMEOUT" default:"10s"`

	// Генерация задач из шаблонов при создании поездки
	GenerateTemplatesEnabled bool `envconfig:"GENERATE_TEMPLATES_ENABLED" default:"false"`
}

func (c *Config) GetLogLevel() slog.Level {
	switch strings.ToLower(c.AppConfig.LogLevel) {
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
