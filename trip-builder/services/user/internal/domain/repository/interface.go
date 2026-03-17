package repository

import (
	"context"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/models"
	"github.com/google/uuid"
)

type ProfileRepository interface {
	Create(ctx context.Context, userID uuid.UUID, firstName, lastName string) error
	GetByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error)
	Update(ctx context.Context, userID uuid.UUID, updates *models.UpdateProfileRequest) error
	UpdateAvatar(ctx context.Context, userID uuid.UUID, avatarURL string) error
	Exists(ctx context.Context, userID uuid.UUID) (bool, error)
}

type StatsRepository interface {
	Create(ctx context.Context, userID uuid.UUID) error
	GetByID(ctx context.Context, userID uuid.UUID) (*models.Stats, error)
	IncrementRoutesCreated(ctx context.Context, userID uuid.UUID) error
	IncrementEventsDone(ctx context.Context, userID uuid.UUID) error
	IncrementCountriesVisited(ctx context.Context, userID uuid.UUID, count int) error
	IncrementPhotosUploaded(ctx context.Context, userID uuid.UUID) error
	IncrementFollowers(ctx context.Context, userID uuid.UUID) error
	DecrementFollowers(ctx context.Context, userID uuid.UUID) error
	IncrementFollowing(ctx context.Context, userID uuid.UUID) error
	DecrementFollowing(ctx context.Context, userID uuid.UUID) error
}

type AchievementRepository interface {
	Create(ctx context.Context, userID uuid.UUID, achievementType string, title, description string) error
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Achievement, error)
	HasAchievement(ctx context.Context, userID uuid.UUID, achievementType string) (bool, error)
}

type FollowRepository interface {
	Follow(ctx context.Context, followerID, followeeID uuid.UUID) error
	Unfollow(ctx context.Context, followerID, followeeID uuid.UUID) error
	IsFollowing(ctx context.Context, followerID, followeeID uuid.UUID) (bool, error)
	GetFollowers(ctx context.Context, userID uuid.UUID, limit, offset int) ([]uuid.UUID, error)
	GetFollowing(ctx context.Context, userID uuid.UUID, limit, offset int) ([]uuid.UUID, error)
	CountFollowers(ctx context.Context, userID uuid.UUID) (int, error)
	CountFollowing(ctx context.Context, userID uuid.UUID) (int, error)
}