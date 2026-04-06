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

	// MinIO
	MinIOEndpoint  string `envconfig:"MINIO_ENDPOINT" required:"true"`
	MinIOAccessKey string `envconfig:"MINIO_ACCESS_KEY" required:"true"`
	MinIOSecretKey string `envconfig:"MINIO_SECRET_KEY" required:"true"`
	MinIOBucket    string `envconfig:"MINIO_BUCKET" default:"trip-builder"`
	MinIOUseSSL    bool   `envconfig:"MINIO_USE_SSL" default:"false"`
	PublicMediaURL string `envconfig:"PUBLIC_MEDIA_URL" required:"true"`

	// Ограничения
	MaxImageSize int64 `envconfig:"MAX_IMAGE_SIZE" default:"10485760"`
	MaxVideoSize int64 `envconfig:"MAX_VIDEO_SIZE" default:"104857600"`

	// Kafka топики
	MediaUploadedTopic string `envconfig:"KAFKA_TOPIC_MEDIA_UPLOADED" default:"media.uploaded"`

	// URL других сервисов
	EventServiceURL string `envconfig:"EVENT_SERVICE_URL" required:"true"`

	// Настройки миграций
	MigrationEnabled bool `envconfig:"MIGRATION_ENABLED" default:"true"`

	// Таймауты
	HTTPClientTimeout time.Duration `envconfig:"HTTP_CLIENT_TIMEOUT" default:"30s"`
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
