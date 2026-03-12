package health

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

type Config struct {
	RedisCheckInterval  time.Duration
	KafkaCheckInterval  time.Duration
	KafkaDialTimeout    time.Duration
	RedisPingTimeout    time.Duration
}

type Checker struct {
	log          *logger.Logger
	redisClient  *redis.Client
	kafkaBrokers []string
	config       Config
}

func NewChecker(log *logger.Logger, redisClient *redis.Client, kafkaBrokers []string, config Config) *Checker {
	// Значения по умолчанию, если не заданы
	if config.RedisCheckInterval == 0 {
		config.RedisCheckInterval = 5 * time.Minute
	}
	if config.KafkaCheckInterval == 0 {
		config.KafkaCheckInterval = 5 * time.Minute
	}
	if config.KafkaDialTimeout == 0 {
		config.KafkaDialTimeout = 3 * time.Second
	}
	if config.RedisPingTimeout == 0 {
		config.RedisPingTimeout = 3 * time.Second
	}

	return &Checker{
		log:          log,
		redisClient:  redisClient,
		kafkaBrokers: kafkaBrokers,
		config:       config,
	}
}

func (c *Checker) Start(ctx context.Context) {
	// Запускаем отдельные горутины для каждого checker'а со своим интервалом
	go c.startRedisChecker(ctx)
	go c.startKafkaChecker(ctx)
}

func (c *Checker) startRedisChecker(ctx context.Context) {
	ticker := time.NewTicker(c.config.RedisCheckInterval)
	defer ticker.Stop()

	// Первая проверка сразу
	c.checkRedis(ctx)

	for {
		select {
		case <-ctx.Done():
			c.log.Debug("Остановка Redis health checker")
			return
		case <-ticker.C:
			c.checkRedis(ctx)
		}
	}
}

func (c *Checker) startKafkaChecker(ctx context.Context) {
	ticker := time.NewTicker(c.config.KafkaCheckInterval)
	defer ticker.Stop()

	// Первая проверка сразу
	c.checkKafka(ctx)

	for {
		select {
		case <-ctx.Done():
			c.log.Debug("Остановка Kafka health checker")
			return
		case <-ticker.C:
			c.checkKafka(ctx)
		}
	}
}

func (c *Checker) checkRedis(ctx context.Context) {
	if c.redisClient == nil {
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, c.config.RedisPingTimeout)
	defer cancel()

	_, err := c.redisClient.Ping(checkCtx).Result()
	if err != nil {
		c.log.Error("Redis недоступен", "error", err)
	} else {
		c.log.Debug("Redis доступен")
	}
}

func (c *Checker) checkKafka(ctx context.Context) {
	if len(c.kafkaBrokers) == 0 {
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, c.config.KafkaDialTimeout)
	defer cancel()

	conn, err := kafka.DialContext(checkCtx, "tcp", c.kafkaBrokers[0])
	if err != nil {
		c.log.Error("Kafka недоступна", "error", err, "broker", c.kafkaBrokers[0])
		return
	}
	defer conn.Close()

	_, err = conn.Brokers()
	if err != nil {
		c.log.Error("Kafka: ошибка получения списка брокеров", "error", err)
	} else {
		c.log.Debug("Kafka доступна")
	}
}