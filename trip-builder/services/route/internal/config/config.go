package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	config.AppConfig

	// Kafka топики
	RouteCreatedTopic string `envconfig:"KAFKA_TOPIC_ROUTE_CREATED" default:"route.created"`
	RouteUpdatedTopic string `envconfig:"KAFKA_TOPIC_ROUTE_UPDATED" default:"route.updated"`
	RouteDeletedTopic string `envconfig:"KAFKA_TOPIC_ROUTE_DELETED" default:"route.deleted"`

	// Настройки миграций
	MigrationEnabled bool `envconfig:"MIGRATION_ENABLED" default:"true"`
	MigrationVersion uint `envconfig:"MIGRATION_VERSION" default:"0"`

	// URL других сервисов (если понадобятся)
	EventServiceURL string `envconfig:"EVENT_SERVICE_URL" default:"http://event:8084"`
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
