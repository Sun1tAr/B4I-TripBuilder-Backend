package repository

import (
	"context"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/domain/models"
	"github.com/google/uuid"
)

// TransferRepository интерфейс для работы с перемещениями маршрута
type TransferRepository interface {
	// Создание одного перемещения
	Create(ctx context.Context, transfer *models.Transfer) error

	// Создание нескольких перемещений в транзакции
	CreateBatch(ctx context.Context, transfers []models.Transfer) error

	// Получение всех перемещений маршрута
	GetByRouteID(ctx context.Context, routeID uuid.UUID) ([]models.Transfer, error)

	// Получение перемещения по ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.Transfer, error)

	// Обновление перемещения
	Update(ctx context.Context, transfer *models.Transfer) error

	// Удаление всех перемещений маршрута
	DeleteByRouteID(ctx context.Context, routeID uuid.UUID) error

	// Удаление одного перемещения
	Delete(ctx context.Context, id uuid.UUID) error

	// Замена всех перемещений маршрута (в транзакции)
	ReplaceForRoute(ctx context.Context, routeID uuid.UUID, transfers []models.Transfer) error
}