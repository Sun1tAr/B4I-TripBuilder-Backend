package repository

import (
	"context"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/models"
	"github.com/google/uuid"
)

// NotificationRepository интерфейс для работы с уведомлениями
type NotificationRepository interface {
	// Создание уведомления
	Create(ctx context.Context, notification *models.Notification) error
	// Создание нескольких уведомлений в одной транзакции
	CreateBatch(ctx context.Context, notifications []*models.Notification) error

	// Получение уведомлений
	GetByID(ctx context.Context, id uuid.UUID) (*models.Notification, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Notification, int64, error)
	GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error)

	// Обновление статуса прочтения
	MarkAsRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	MarkAllAsRead(ctx context.Context, userID uuid.UUID) (int64, error)

	// Удаление (для cleanup)
	DeleteOlderThan(ctx context.Context, days int) (int64, error) // ← НОВЫЙ МЕТОД
}

// DeviceRepository интерфейс для работы с устройствами
type DeviceRepository interface {
	// Регистрация устройства
	Register(ctx context.Context, device *models.UserDevice) error
	// Удаление устройства
	Unregister(ctx context.Context, userID uuid.UUID, token string) error

	// Получение устройств
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.UserDevice, error)
	GetByToken(ctx context.Context, token string) (*models.UserDevice, error)

	// Обновление last_used
	UpdateLastUsed(ctx context.Context, userID uuid.UUID, token string) error

	// Удаление неактивных устройств
	DeleteInactive(ctx context.Context, days int) (int64, error) // ← НОВЫЙ МЕТОД
}

// TemplateRepository интерфейс для работы с шаблонами писем
type TemplateRepository interface {
	// Получение шаблона
	GetByType(ctx context.Context, templateType, language string) (*models.EmailTemplate, error)

	// Получение всех шаблонов определенного типа (для fallback)
	GetAllByType(ctx context.Context, templateType string) ([]*models.EmailTemplate, error)

	// Создание/обновление шаблона (для админки в будущем)
	Upsert(ctx context.Context, template *models.EmailTemplate) error
}
