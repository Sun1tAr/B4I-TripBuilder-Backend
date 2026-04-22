package worker

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type CleanupWorker struct {
	notificationRepo repository.NotificationRepository
	deviceRepo       repository.DeviceRepository
	log              *logger.Logger
	interval         time.Duration
	retentionDays    int
}

func NewCleanupWorker(
	notificationRepo repository.NotificationRepository,
	deviceRepo repository.DeviceRepository,
	log *logger.Logger,
	interval time.Duration,
	retentionDays int,
) *CleanupWorker {
	log.Debug("CleanupWorker: инициализация",
		"interval", interval,
		"retention_days", retentionDays,
	)

	return &CleanupWorker{
		notificationRepo: notificationRepo,
		deviceRepo:       deviceRepo,
		log:              log,
		interval:         interval,
		retentionDays:    retentionDays,
	}
}

func (w *CleanupWorker) Start(ctx context.Context) {
	w.log.Info("CleanupWorker: запуск")

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Первая очистка сразу
	w.cleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			w.log.Info("CleanupWorker: остановка")
			return
		case <-ticker.C:
			w.cleanup(ctx)
		}
	}
}

func (w *CleanupWorker) cleanup(ctx context.Context) {
	w.log.Debug("CleanupWorker: начало очистки")

	// Очистка старых уведомлений (храним 90 дней по умолчанию)
	deletedNotifications, err := w.notificationRepo.DeleteOlderThan(ctx, w.retentionDays)
	if err != nil {
		w.log.Error("CleanupWorker: ошибка удаления старых уведомлений",
			"error", err,
		)
	} else {
		w.log.Info("CleanupWorker: старые уведомления удалены",
			"count", deletedNotifications,
			"retention_days", w.retentionDays,
		)
	}

	// Очистка неактивных устройств (не использовались 60 дней)
	deletedDevices, err := w.deviceRepo.DeleteInactive(ctx, 60)
	if err != nil {
		w.log.Error("CleanupWorker: ошибка удаления неактивных устройств",
			"error", err,
		)
	} else {
		w.log.Info("CleanupWorker: неактивные устройства удалены",
			"count", deletedDevices,
		)
	}
}
