package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/redis/go-redis/v9"
)

type EmailRateLimiter struct {
	client *redis.Client
	log    *logger.Logger
	limit  int
}

func NewEmailRateLimiter(client *redis.Client, limit int, log *logger.Logger) *EmailRateLimiter {
	log.Debug("EmailRateLimiter: инициализация",
		"daily_limit", limit,
	)
	return &EmailRateLimiter{
		client: client,
		log:    log,
		limit:  limit,
	}
}

// Allow проверяет, не превышен ли лимит писем для пользователя за сегодня
func (r *EmailRateLimiter) Allow(ctx context.Context, userID string) (bool, error) {
	r.log.Debug("EmailRateLimiter.Allow: проверка лимита",
		"user_id", userID,
	)

	today := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("email_limit:%s:%s", userID, today)

	// Получаем текущее значение
	count, err := r.client.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		r.log.Error("EmailRateLimiter.Allow: ошибка получения из Redis",
			"error", err,
			"user_id", userID,
		)
		return false, err
	}

	r.log.Debug("EmailRateLimiter.Allow: текущий счетчик",
		"user_id", userID,
		"count", count,
		"limit", r.limit,
	)

	// Проверяем лимит
	if count >= r.limit {
		r.log.Info("EmailRateLimiter.Allow: лимит превышен",
			"user_id", userID,
			"count", count,
			"limit", r.limit,
		)
		return false, nil
	}

	// Инкрементируем и устанавливаем TTL до конца дня
	pipe := r.client.Pipeline()
	incr := pipe.Incr(ctx, key)

	// Устанавливаем TTL до полуночи
	now := time.Now()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	ttl := tomorrow.Sub(now)
	pipe.Expire(ctx, key, ttl)

	_, err = pipe.Exec(ctx)
	if err != nil {
		r.log.Error("EmailRateLimiter.Allow: ошибка инкремента",
			"error", err,
			"user_id", userID,
		)
		return false, err
	}

	newCount := int(incr.Val())
	r.log.Debug("EmailRateLimiter.Allow: разрешено",
		"user_id", userID,
		"new_count", newCount,
		"ttl", ttl,
	)

	return true, nil
}

// GetRemaining возвращает оставшееся количество писем на сегодня
func (r *EmailRateLimiter) GetRemaining(ctx context.Context, userID string) (int, error) {
	today := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("email_limit:%s:%s", userID, today)

	count, err := r.client.Get(ctx, key).Int()
	if err == redis.Nil {
		return r.limit, nil
	}
	if err != nil {
		return 0, err
	}

	remaining := r.limit - count
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// Reset сбрасывает счетчик для пользователя (для тестирования)
func (r *EmailRateLimiter) Reset(ctx context.Context, userID string) error {
	today := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("email_limit:%s:%s", userID, today)
	return r.client.Del(ctx, key).Err()
}
