package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/config"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	config.AppConfig
	JWTSecret     string `envconfig:"JWT_SECRET" required:"true"`
	TokenLifetime int    `envconfig:"TOKEN_LIFETIME" default:"15"`
	BCryptCost    int    `envconfig:"BCRYPT_COST" default:"10"`
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
		slog.Error("ошибка загрузки конфигурации", "error", err)
		os.Exit(1)
	}
	return cfg
}