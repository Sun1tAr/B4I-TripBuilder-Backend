package repository

import (
	"context"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/domain/models"
	"github.com/google/uuid"
)

type RouteRepository interface {
	// Создание маршрута
	Create(ctx context.Context, route *models.Route, waypoints []models.Waypoint) error
	CreateWithWaypoints(ctx context.Context, route *models.Route, waypoints []models.Waypoint, transfers []models.Transfer) error // ← ОБНОВЛЯЕМ

	// Получение маршрута по ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.Route, error)
	GetByIDWithDetails(ctx context.Context, id uuid.UUID) (*models.Route, []models.Waypoint, []models.Transfer, error) // ← ОБНОВЛЯЕМ

	// Получение списка маршрутов
	ListPublic(ctx context.Context, limit, offset int, sort string, filters map[string]interface{}) ([]models.Route, int64, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Route, int64, error)
	ListByTag(ctx context.Context, tagName string, limit, offset int) ([]models.Route, int64, error)
	GetPopular(ctx context.Context, limit int) ([]models.Route, error)

	// Обновление маршрута
	Update(ctx context.Context, route *models.Route, waypoints []models.Waypoint) error
	UpdateWithTransfers(ctx context.Context, route *models.Route, waypoints []models.Waypoint, transfers []models.Transfer) error // ← ДОБАВЛЯЕМ

	// Удаление маршрута
	Delete(ctx context.Context, id uuid.UUID) error

	// Проверка владельца
	IsOwner(ctx context.Context, routeID, userID uuid.UUID) (bool, error)

	// Существует ли маршрут
	Exists(ctx context.Context, id uuid.UUID) (bool, error)

	// Инкремент счётчика событий
	IncrementEventsCount(ctx context.Context, routeID uuid.UUID) error
}

type TagRepository interface {
	// Создание тега
	Create(ctx context.Context, name string) (*models.Tag, error)

	// Получение тега по имени
	GetByName(ctx context.Context, name string) (*models.Tag, error)

	// Получение или создание тега
	GetOrCreate(ctx context.Context, name string) (*models.Tag, error)

	// Получение всех тегов
	GetAll(ctx context.Context, limit, offset int) ([]models.Tag, int64, error)

	// Получение тегов маршрута
	GetByRouteID(ctx context.Context, routeID uuid.UUID) ([]models.Tag, error)

	// Привязка тегов к маршруту
	AttachToRoute(ctx context.Context, routeID uuid.UUID, tagIDs []uuid.UUID) error

	// Полная замена тегов маршрута
	ReplaceForRoute(ctx context.Context, routeID uuid.UUID, tagIDs []uuid.UUID) error

	// Инкремент счётчика использования
	IncrementCount(ctx context.Context, tagID uuid.UUID) error

	// Декремент счётчика использования
	DecrementCount(ctx context.Context, tagID uuid.UUID) error
}

type LikeRepository interface {
	// Поставить лайк
	Like(ctx context.Context, userID, routeID uuid.UUID) error

	// Убрать лайк
	Unlike(ctx context.Context, userID, routeID uuid.UUID) error

	// Проверка, лайкнул ли пользователь
	IsLiked(ctx context.Context, userID, routeID uuid.UUID) (bool, error)

	// Получить количество лайков
	CountByRouteID(ctx context.Context, routeID uuid.UUID) (int, error)

	// Получить список пользователей, лайкнувших маршрут
	GetUsersByRouteID(ctx context.Context, routeID uuid.UUID, limit, offset int) ([]uuid.UUID, int64, error)
}
