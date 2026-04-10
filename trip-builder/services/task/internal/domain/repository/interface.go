package repository

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/domain/models"
	"github.com/google/uuid"
)

type TaskRepository interface {
	Create(ctx context.Context, task *models.Task) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error)
	GetByEventID(ctx context.Context, eventID uuid.UUID) ([]models.Task, error)
	List(ctx context.Context, eventID *uuid.UUID, userID uuid.UUID, isOwner bool, limit, offset int) ([]models.Task, int64, error) // изменён
	Update(ctx context.Context, task *models.Task) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, completedAt *time.Time) error
	CancelByEventID(ctx context.Context, eventID uuid.UUID) error
}

type TaskCommentRepository interface {
	Create(ctx context.Context, comment *models.TaskComment) error
	GetByTaskID(ctx context.Context, taskID uuid.UUID, limit, offset int) ([]models.TaskComment, int64, error)
	CountByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
	IsAuthor(ctx context.Context, commentID, userID uuid.UUID) (bool, error)
}

type TaskTemplateRepository interface {
	List(ctx context.Context) ([]models.TaskTemplate, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.TaskTemplate, error)
}
