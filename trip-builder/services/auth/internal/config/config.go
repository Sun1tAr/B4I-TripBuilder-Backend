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
	JWTSecret       string        `envconfig:"JWT_SECRET" required:"true"`
	TokenLifetime   time.Duration `envconfig:"TOKEN_LIFETIME" default:"15m"`
	RefreshLifetime time.Duration `envconfig:"REFRESH_LIFETIME" default:"168h"` // 7 дней
	BCryptCost      int           `envconfig:"BCRYPT_COST" default:"10"`
	RateLimitLogin  int           `envconfig:"RATE_LIMIT_LOGIN" default:"5"` // попыток в минуту
	RateLimitWindow time.Duration `envconfig:"RATE_LIMIT_WINDOW" default:"1m"`

	PasswordResetTopic string `envconfig:"KAFKA_TOPIC_PASSWORD_RESET" default:"password.reset"`

	// Время жизни токенов
	VerificationTokenLifetime  time.Duration `envconfig:"VERIFICATION_TOKEN_LIFETIME" default:"24h"`
	ResetPasswordTokenLifetime time.Duration `envconfig:"RESET_PASSWORD_TOKEN_LIFETIME" default:"1h"`

	// Kafka топики
	UserCreatedTopic string `envconfig:"KAFKA_TOPIC_USER_CREATED" default:"user.created"`

	// Cron интервалы
	TokenCleanupInterval time.Duration `envconfig:"TOKEN_CLEANUP_INTERVAL" default:"1h"`

	// Health check интервалы
	RedisHealthInterval time.Duration `envconfig:"REDIS_HEALTH_INTERVAL" default:"5m"`
	KafkaHealthInterval time.Duration `envconfig:"KAFKA_HEALTH_INTERVAL" default:"5m"`
	KafkaDialTimeout    time.Duration `envconfig:"KAFKA_DIAL_TIMEOUT" default:"3s"`
	RedisPingTimeout    time.Duration `envconfig:"REDIS_PING_TIMEOUT" default:"3s"`

	// URL других сервисов
	UserServiceURL string `envconfig:"USER_SERVICE_URL" required:"true"`

	// Настройки миграций
	MigrationEnabled bool `envconfig:"MIGRATION_ENABLED" default:"true"`
	MigrationVersion uint `envconfig:"MIGRATION_VERSION" default:"0"` // 0 = последняя
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
