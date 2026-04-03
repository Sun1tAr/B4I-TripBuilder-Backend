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

	// Redis для pub/sub
	Redis config.Redis

	// Kafka топики
	ChatMessageTopic       string `envconfig:"KAFKA_TOPIC_CHAT_MESSAGE" required:"true"`
	WaypointVisitedTopic   string `envconfig:"KAFKA_TOPIC_WAYPOINT_VISITED" required:"true"`
	TaskCompletedTopic     string `envconfig:"KAFKA_TOPIC_TASK_COMPLETED" required:"true"`
	MediaUploadedTopic     string `envconfig:"KAFKA_TOPIC_MEDIA_UPLOADED" required:"true"`
	ParticipantJoinedTopic string `envconfig:"KAFKA_TOPIC_PARTICIPANT_JOINED" required:"true"`

	// URL других сервисов
	EventServiceURL string `envconfig:"EVENT_SERVICE_URL" required:"true"`

	// Настройки миграций
	MigrationEnabled bool `envconfig:"MIGRATION_ENABLED" default:"true"`

	// Таймауты
	HTTPClientTimeout time.Duration `envconfig:"HTTP_CLIENT_TIMEOUT" default:"10s"`

	// WebSocket настройки
	WSReadBufferSize  int      `envconfig:"WS_READ_BUFFER_SIZE" default:"1024"`
	WSWriteBufferSize int      `envconfig:"WS_WRITE_BUFFER_SIZE" default:"1024"`
	AllowedOrigins    []string `envconfig:"WS_ALLOWED_ORIGINS" default:"http://localhost:3000"`
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
