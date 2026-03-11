package config

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
)

type Database struct {
	Host     string `envconfig:"DB_HOST" default:"localhost"`
	Port     int    `envconfig:"DB_PORT" default:"5432"`
	User     string `envconfig:"DB_USER" required:"true"`
	Password string `envconfig:"DB_PASSWORD" required:"true"`
	Name     string `envconfig:"DB_NAME" required:"true"`
	SSLMode  string `envconfig:"DB_SSLMODE" default:"disable"`
}

type Kafka struct {
	Brokers []string `envconfig:"KAFKA_BROKERS" default:"localhost:9092"`
	GroupID string   `envconfig:"KAFKA_GROUP_ID" required:"true"`
}

type Redis struct {
	Addr     string `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	Password string `envconfig:"REDIS_PASSWORD" default:""`
	DB       int    `envconfig:"REDIS_DB" default:"0"`
}

// AppConfig – общая структура для всех сервисов (каждый сервис может расширить свою)
type AppConfig struct {
	ServiceName string `envconfig:"SERVICE_NAME" required:"true"`
	HTTPPort    int    `envconfig:"HTTP_PORT" default:"8080"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`

	Database Database
	Kafka    Kafka
	Redis    Redis
}

func Load() (*AppConfig, error) {
	var cfg AppConfig
	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &cfg, nil
}