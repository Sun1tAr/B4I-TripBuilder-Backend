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

	// Kafka топики для консьюмеров
	UserCreatedTopic   string `envconfig:"KAFKA_TOPIC_USER_CREATED" default:"user.created"`
	EventInviteTopic   string `envconfig:"KAFKA_TOPIC_EVENT_INVITE" default:"event.invite"`
	TaskAssignedTopic  string `envconfig:"KAFKA_TOPIC_TASK_ASSIGNED" default:"task.assigned"`
	ChatMessageTopic   string `envconfig:"KAFKA_TOPIC_CHAT_MESSAGE" default:"chat.message"`
	EventReminderTopic string `envconfig:"KAFKA_TOPIC_EVENT_REMINDER" default:"event.reminder"`

	// URL других сервисов
	UserServiceURL  string `envconfig:"USER_SERVICE_URL" required:"true"`
	EventServiceURL string `envconfig:"EVENT_SERVICE_URL" required:"true"`
	ChatServiceURL  string `envconfig:"CHAT_SERVICE_URL" required:"true"`

	// SMTP настройки
	SMTPHost     string `envconfig:"SMTP_HOST" default:"localhost"`
	SMTPPort     string `envconfig:"SMTP_PORT" default:"1025"`
	SMTPUsername string `envconfig:"SMTP_USERNAME" default:""`
	SMTPPassword string `envconfig:"SMTP_PASSWORD" default:""`
	SMTPFrom     string `envconfig:"SMTP_FROM" default:"noreply@tripbuilder.local"`

	// Настройки Redis для rate limiting
	Redis config.Redis

	// Rate limiting
	EmailDailyLimit int           `envconfig:"EMAIL_DAILY_LIMIT" default:"5"`
	RateLimitWindow time.Duration `envconfig:"RATE_LIMIT_WINDOW" default:"24h"`

	// Настройки миграций
	MigrationEnabled bool `envconfig:"MIGRATION_ENABLED" default:"true"`
	MigrationVersion uint `envconfig:"MIGRATION_VERSION" default:"0"`

	// Таймауты HTTP клиента
	HTTPClientTimeout time.Duration `envconfig:"HTTP_CLIENT_TIMEOUT" default:"10s"`

	// ========== URL для ссылок в письмах ==========
	// Базовый URL фронтенда (используется по умолчанию если конкретный URL не задан)
	BaseURL string `envconfig:"BASE_URL" default:"http://localhost:3000"`

	// Конкретные URL для разных типов ссылок
	VerifyEmailURL string `envconfig:"VERIFY_EMAIL_URL" default:""` // /verify-email?token=
	EventInviteURL string `envconfig:"EVENT_INVITE_URL" default:""` // /events/invite/
	TaskURL        string `envconfig:"TASK_URL" default:""`         // /events/{eventId}/tasks/
	ChatURL        string `envconfig:"CHAT_URL" default:""`         // /events/{eventId}/chat
	EventURL       string `envconfig:"EVENT_URL" default:""`        // /events/

	// Cleanup настройки
	NotificationRetentionDays int           `envconfig:"NOTIFICATION_RETENTION_DAYS" default:"90"`
	CleanupInterval           time.Duration `envconfig:"CLEANUP_INTERVAL" default:"24h"`
}

// GetVerifyEmailURL возвращает URL для подтверждения email
// Плейсхолдер: %s = token
func (c *Config) GetVerifyEmailURL(token string) string {
	if c.VerifyEmailURL == "" {
		return fmt.Sprintf("%s/verify-email?token=%s", c.BaseURL, token)
	}
	return fmt.Sprintf(c.VerifyEmailURL, token)
}

// GetEventInviteURL возвращает URL для приглашения в поездку
// Плейсхолдер: %s = eventID
func (c *Config) GetEventInviteURL(eventID string) string {
	if c.EventInviteURL == "" {
		return fmt.Sprintf("%s/events/invite/%s", c.BaseURL, eventID)
	}
	return fmt.Sprintf(c.EventInviteURL, eventID)
}

// GetTaskURL возвращает URL для задачи
// Плейсхолдеры: %s = eventID, %s = taskID (в порядке eventID)
func (c *Config) GetTaskURL(eventID, taskID string) string {
	if c.TaskURL == "" {
		return fmt.Sprintf("%s/events/%s", c.BaseURL, eventID)
	}
	return fmt.Sprintf(c.TaskURL, eventID)
}

// GetChatURL возвращает URL для чата
// Плейсхолдер: %s = eventID
func (c *Config) GetChatURL(eventID string) string {
	if c.ChatURL == "" {
		return fmt.Sprintf("%s/events/%s/chat", c.BaseURL, eventID)
	}
	return fmt.Sprintf(c.ChatURL, eventID)
}

// GetEventURL возвращает URL для поездки
// Плейсхолдер: %s = eventID
func (c *Config) GetEventURL(eventID string) string {
	if c.EventURL == "" {
		return fmt.Sprintf("%s/events/%s", c.BaseURL, eventID)
	}
	return fmt.Sprintf(c.EventURL, eventID)
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
