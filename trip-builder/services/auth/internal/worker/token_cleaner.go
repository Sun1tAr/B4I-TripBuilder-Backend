package worker

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type TokenCleaner struct {
	tokenRepo repository.RefreshTokenRepository
	log       *logger.Logger
	interval  time.Duration
}

func NewTokenCleaner(tokenRepo repository.RefreshTokenRepository, log *logger.Logger, interval time.Duration) *TokenCleaner {
	return &TokenCleaner{
		tokenRepo: tokenRepo,
		log:       log,
		interval:  interval,
	}
}

func (c *TokenCleaner) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Первая очистка сразу
	c.clean(ctx)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Остановка очистки токенов")
			return
		case <-ticker.C:
			c.clean(ctx)
		}
	}
}

func (c *TokenCleaner) clean(ctx context.Context) {
	err := c.tokenRepo.DeleteExpired(ctx)
	if err != nil {
		c.log.Error("Ошибка при удалении просроченных токенов", "error", err)
	} else {
		c.log.Debug("Просроченные токены удалены")
	}
}