package repository

import (
	"context"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/models"
	"github.com/google/uuid"
	"time"
)

// EventTransferRepository интерфейс для работы с перемещениями поездки
type EventTransferRepository interface {
	// Создание одного перемещения
	Create(ctx context.Context, transfer *models.EventTransfer) error

	// Создание нескольких перемещений в транзакции
	CreateBatch(ctx context.Context, transfers []models.EventTransfer) error

	// Получение всех перемещений поездки
	GetByEventID(ctx context.Context, eventID uuid.UUID) ([]models.EventTransfer, error)

	// Получение перемещения по ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.EventTransfer, error)

	// Получение перемещения между двумя точками
	GetByWaypoints(ctx context.Context, fromWaypointID, toWaypointID uuid.UUID) (*models.EventTransfer, error)

	// Обновление перемещения
	Update(ctx context.Context, transfer *models.EventTransfer) error

	// Обновление фактического времени
	UpdateActualTimes(ctx context.Context, id uuid.UUID, actualStart, actualEnd *time.Time) error

	// Удаление всех перемещений поездки
	DeleteByEventID(ctx context.Context, eventID uuid.UUID) error

	// Удаление одного перемещения
	Delete(ctx context.Context, id uuid.UUID) error
}