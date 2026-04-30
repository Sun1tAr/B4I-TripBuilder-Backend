package repository

import (
	"context"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/domain/models"
	"github.com/google/uuid"
)

type MediaRepository interface {
	Create(ctx context.Context, media *models.Media) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Media, error)
	GetByObjectKey(ctx context.Context, objectKey string) (*models.Media, error) // новый метод
	GetByEventID(ctx context.Context, eventID uuid.UUID, limit, offset int) ([]models.Media, int64, error)
	GetByWaypointID(ctx context.Context, waypointID uuid.UUID, limit, offset int) ([]models.Media, int64, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Media, int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByEventID(ctx context.Context, eventID uuid.UUID) error
	DeleteByWaypointID(ctx context.Context, waypointID uuid.UUID) error
	UpdateThumbnails(ctx context.Context, id uuid.UUID, thumbnailURL, mediumURL string) error
	IsOwner(ctx context.Context, mediaID, userID uuid.UUID) (bool, error)
}
