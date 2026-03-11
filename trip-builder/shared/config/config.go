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
	Name     string `envconfig:"DB_NAME" required:"true"`   // общая база, например "trip-builder"
	Schema   string `envconfig:"DB_SCHEMA" required:"true"` // схема сервиса, например "tpb_auth"
	SSLMode  string `envconfig:"DB_SSLMODE" default:"disable"`
}

func (d Database) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s search_path=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.Schema, d.SSLMode)
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

type AppConfig struct {
	ServiceName string `envconfig:"SERVICE_NAME" required:"true"`
	HTTPPort    string `envconfig:"HTTP_PORT" default:"8080"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`
	LogFormat   string `envconfig:"LOG_FORMAT" default:"console"`

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