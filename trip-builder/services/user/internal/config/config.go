package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/retry"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	config.AppConfig
	// JWTSecret удалён - user service не нужен
	AvatarMaxSize    int64         `envconfig:"AVATAR_MAX_SIZE" default:"5242880"`
	AllowedImageTypes []string     `envconfig:"ALLOWED_IMAGE_TYPES" default:"image/jpeg,image/png,image/gif"`

	// URLs других сервисов
	MediaServiceURL  string        `envconfig:"MEDIA_SERVICE_URL" required:"true"`
	EventServiceURL  string        `envconfig:"EVENT_SERVICE_URL" required:"true"`

	// Kafka для retry
	KafkaBrokers     []string      `envconfig:"KAFKA_BROKERS" default:"localhost:9092"`
	RetryTopic       string        `envconfig:"RETRY_TOPIC" default:"user.events.retry"`
	DLQTopic         string        `envconfig:"DLQ_TOPIC" default:"user.events.dlq"`
	MaxRetries       int           `envconfig:"RETRY_MAX_ATTEMPTS" default:"5"`
	RetryInitialBackoff time.Duration `envconfig:"RETRY_INITIAL_BACKOFF" default:"100ms"`
	RetryMaxBackoff  time.Duration `envconfig:"RETRY_MAX_BACKOFF" default:"30s"`

	// Настройки миграций
	MigrationEnabled bool   `envconfig:"MIGRATION_ENABLED" default:"true"`
	MigrationVersion uint   `envconfig:"MIGRATION_VERSION" default:"0"`
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
		slog.Error("Ошибка загрузки конфигурации", "error", "dont upload .env file log level")
		os.Exit(1)
		return slog.LevelInfo
	}
}

func (c *Config) GetRetryConfig() retry.RetryConfig {
	slog.Info("Создание конфига Retry: ", "KafkaBrokers", c.KafkaBrokers)
	return retry.RetryConfig{
		Brokers:        c.KafkaBrokers,
		RetryTopic:     c.RetryTopic,
		DLQTopic:       c.DLQTopic,
		MaxRetries:     c.MaxRetries,
		InitialBackoff: c.RetryInitialBackoff,
		MaxBackoff:     c.RetryMaxBackoff,
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