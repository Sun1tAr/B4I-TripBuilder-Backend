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
	CreateWithWaypoints(ctx context.Context, event *models.Event, waypoints []models.Waypoint) error
	CreateWithWaypointsAndTransfers(ctx context.Context, event *models.Event, waypoints []models.Waypoint, transfers []models.EventTransfer) error // ← ДОБАВЛЯЕМ

	// Получение
	GetByID(ctx context.Context, id uuid.UUID) (*models.Event, error)
	GetByIDWithDetails(ctx context.Context, id uuid.UUID) (*models.Event, []models.Waypoint, []models.EventTransfer, error) // ← ОБНОВЛЯЕМ

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
	GetParticipantsStats(ctx context.Context, eventIDs []uuid.UUID) (map[uuid.UUID]EventParticipantsStats, error)
}

type ParticipantRepository interface {
	// Добавление участника
	Add(ctx context.Context, participant *models.Participant) error
	AddBatch(ctx context.Context, eventID uuid.UUID, userIDs []uuid.UUID) error

	// Получение
	GetByEventID(ctx context.Context, eventID uuid.UUID) ([]models.Participant, error)
	GetByUserAndEvent(ctx context.Context, eventID, userID uuid.UUID) (*models.Participant, error)
	GetByInviteID(ctx context.Context, inviteID uuid.UUID) (*models.Participant, error)

	// Получение приглашений (approved_by_owner = true AND approved_by_user IS NULL)
	GetInvitesByUserID(ctx context.Context, userID uuid.UUID) ([]models.Invite, error)

	// Обновление
	UpdateApproval(ctx context.Context, inviteID uuid.UUID, approveByOwner, approveByUser *bool) error

	// Удаление
	Remove(ctx context.Context, eventID, userID uuid.UUID) error

	// Проверки
	IsParticipant(ctx context.Context, eventID, userID uuid.UUID) (bool, error)
	IsOwner(ctx context.Context, eventID, userID uuid.UUID) (bool, error)
	CountByEventID(ctx context.Context, eventID uuid.UUID) (int, error)

	// Для связывания незарегистрированных пользователей
	LinkInvitation(ctx context.Context, inviteID uuid.UUID, userID uuid.UUID) error
	FindByInvitedEmail(ctx context.Context, email string) ([]models.Participant, error)
}

type WaypointRepository interface {
	// Создание
	Create(ctx context.Context, waypoint *models.Waypoint) error
	CreateBatch(ctx context.Context, waypoints []models.Waypoint) error

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

// EventParticipantsStats статистика участников события
type EventParticipantsStats struct {
	TotalInvites      int
	TotalParticipants int
}
