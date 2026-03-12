package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client *redis.Client
	log    *logger.Logger
	limit  int           // максимальное число попыток
	window time.Duration // окно в секундах
}

func NewRateLimiter(redisAddr, redisPassword string, redisDB int, limit int, window time.Duration, log *logger.Logger) *RateLimiter {
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	return &RateLimiter{
		client: client,
		log:    log,
		limit:  limit,
		window: window,
	}
}

func (r *RateLimiter) GetLimit() int {
	return r.limit
}

func (r *RateLimiter) GetWindow() time.Duration {
	return r.window
}

func (r *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	redisKey := fmt.Sprintf("rate_limit:login:%s", key)

	// Транзакция: увеличиваем счётчик и устанавливаем TTL при первом обращении
	pipe := r.client.Pipeline()
	incr := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, r.window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		r.log.Error("Ошибка Redis при rate limit", "error", err, "key", key)
		return false, err
	}

	count := incr.Val()
	allowed := count <= int64(r.limit)

	return allowed, nil
}

func (r *RateLimiter) Reset(ctx context.Context, key string) error {
	redisKey := fmt.Sprintf("rate_limit:login:%s", key)
	return r.client.Del(ctx, redisKey).Err()
}

func (r *RateLimiter) Close() error {
	return r.client.Close()
}