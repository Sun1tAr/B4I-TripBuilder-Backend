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
	JWTSecret        string        `envconfig:"JWT_SECRET" required:"true"`
	AvatarMaxSize    int64         `envconfig:"AVATAR_MAX_SIZE" default:"5242880"`
	AllowedImageTypes []string     `envconfig:"ALLOWED_IMAGE_TYPES" default:"image/jpeg,image/png,image/gif"`
	KafkaTopicUserCreated string   `envconfig:"KAFKA_TOPIC_USER_CREATED" default:"user.created"`
	KafkaTopicUserUpdated string   `envconfig:"KAFKA_TOPIC_USER_UPDATED" default:"user.updated"`
	KafkaTopicUserDeleted string   `envconfig:"KAFKA_TOPIC_USER_DELETED" default:"user.deleted"`

	// Настройки миграций
	MigrationEnabled bool   `envconfig:"MIGRATION_ENABLED" default:"true"`
	MigrationVersion uint   `envconfig:"MIGRATION_VERSION" default:"0"` // 0 = последняя
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