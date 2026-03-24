package repository

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/models"
	"github.com/google/uuid"
)

type EventRepository interface {
	// Создание
	Create(ctx context.Context, event *models.Event) error

	// Получение
	GetByID(ctx context.Context, id uuid.UUID) (*models.Event, error)
	GetByIDWithDetails(ctx context.Context, id uuid.UUID) (*models.Event, error)

	// Списки
	ListByUserID(ctx context.Context, userID uuid.UUID, status string, limit, offset int) ([]models.Event, int64, error)
	ListPublic(ctx context.Context, limit, offset int) ([]models.Event, int64, error)

	// Обновление
	Update(ctx context.Context, event *models.Event) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error

	// Удаление
	Delete(ctx context.Context, id uuid.UUID) error

	// Проверки
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
	IsOwner(ctx context.Context, eventID, userID uuid.UUID) (bool, error)

	// Вспомогательные
	GetActiveEvents(ctx context.Context, beforeDate time.Time) ([]uuid.UUID, error)
	GetPlannedEventsForActivation(ctx context.Context, beforeDate time.Time) ([]uuid.UUID, error)

	CreateWithWaypoints(ctx context.Context, event *models.Event, waypoints []models.Waypoint) error
}

type ParticipantRepository interface {
	// Добавление участника
	Add(ctx context.Context, participant *models.Participant) error
	AddBatch(ctx context.Context, eventID uuid.UUID, userIDs []uuid.UUID) error

	// Получение
	GetByEventID(ctx context.Context, eventID uuid.UUID) ([]models.Participant, error)
	GetByUserAndEvent(ctx context.Context, eventID, userID uuid.UUID) (*models.Participant, error)

	// Обновление
	UpdateStatus(ctx context.Context, eventID, userID uuid.UUID, status string) error

	// Удаление
	Remove(ctx context.Context, eventID, userID uuid.UUID) error

	// Проверки
	IsParticipant(ctx context.Context, eventID, userID uuid.UUID) (bool, error)
	IsOwner(ctx context.Context, eventID, userID uuid.UUID) (bool, error)
	CountByEventID(ctx context.Context, eventID uuid.UUID) (int, error)

	// Для приглашений
	FindByInvitedEmail(ctx context.Context, eventID uuid.UUID, email string) (*models.Participant, error)
	LinkInvitedParticipant(ctx context.Context, eventID, userID uuid.UUID, email string) error

	// LinkAllInvitationsByEmail связывает все приглашения с указанным email с пользователем
	LinkAllInvitationsByEmail(ctx context.Context, userID uuid.UUID, email string) error
}

type WaypointRepository interface {
	// Создание
	Create(ctx context.Context, waypoint *models.Waypoint) error

	// Получение
	GetByEventID(ctx context.Context, eventID uuid.UUID) ([]models.Waypoint, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Waypoint, error)

	// Обновление
	Update(ctx context.Context, waypoint *models.Waypoint) error
	UpdateVisit(ctx context.Context, id uuid.UUID, actualStart, actualEnd *time.Time) error

	// Удаление
	DeleteByEventID(ctx context.Context, eventID uuid.UUID) error

	// Проверки
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

type MediaRepository interface {
	// Создание
	Create(ctx context.Context, media *models.Media) error

	// Получение
	GetByID(ctx context.Context, id uuid.UUID) (*models.Media, error)
	GetByWaypointID(ctx context.Context, waypointID uuid.UUID, limit, offset int) ([]models.Media, int64, error)
	GetByEventID(ctx context.Context, eventID uuid.UUID, limit, offset int) ([]models.Media, int64, error)

	// Удаление
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByWaypointID(ctx context.Context, waypointID uuid.UUID) error

	// Проверки
	IsOwner(ctx context.Context, mediaID, userID uuid.UUID) (bool, error)
}

type CommentRepository interface {
	// Создание
	Create(ctx context.Context, comment *models.Comment) error

	// Получение
	GetByID(ctx context.Context, id uuid.UUID) (*models.Comment, error)
	GetByWaypointID(ctx context.Context, waypointID uuid.UUID, limit, offset int) ([]models.Comment, int64, error)

	// Обновление
	Update(ctx context.Context, comment *models.Comment) error

	// Удаление
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByWaypointID(ctx context.Context, waypointID uuid.UUID) error

	// Проверки
	IsAuthor(ctx context.Context, commentID, userID uuid.UUID) (bool, error)
}
