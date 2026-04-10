package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/kelseyhightower/envconfig"
)

type ServiceConfig struct {
	AuthURL         string `envconfig:"AUTH_URL" required:"true"`
	UserURL         string `envconfig:"USER_URL" required:"true"`
	RouteURL        string `envconfig:"ROUTE_URL" required:"true"`
	EventURL        string `envconfig:"EVENT_URL" required:"true"`
	TaskURL         string `envconfig:"TASK_URL" required:"true"`
	ChatURL         string `envconfig:"CHAT_URL" required:"true"`
	ChatWSURL       string `envconfig:"CHAT_WS_URL" required:"true"`
	MediaURL        string `envconfig:"MEDIA_URL" required:"true"`
	SearchURL       string `envconfig:"SEARCH_URL" required:"true"`
	NotificationURL string `envconfig:"NOTIFICATION_URL" required:"true"`
}

type Config struct {
	ServiceName string `envconfig:"SERVICE_NAME" required:"true"`
	HTTPPort    string `envconfig:"HTTP_PORT" default:"8090"`
	WSPort      string `envconfig:"WS_PORT" default:"8091"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`
	LogFormat   string `envconfig:"LOG_FORMAT" default:"console"`

	JWTSecret string `envconfig:"JWT_SECRET" required:"true"`

	RateLimitRequests int           `envconfig:"RATE_LIMIT_REQUESTS" default:"100"`
	RateLimitWindow   time.Duration `envconfig:"RATE_LIMIT_WINDOW" default:"1m"`

	CORSAllowedOrigins []string `envconfig:"CORS_ALLOWED_ORIGINS" default:"*"`

	// Health check interval (например, 30s, 1m, 5m)
	HealthCheckInterval time.Duration `envconfig:"HEALTH_CHECK_INTERVAL" default:"30s"`

	Services ServiceConfig
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
