package service

import (
	"context"
	"errors"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/http/handlers/dto"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

var (
	ErrProfileNotFound = errors.New("profile not found")
)

type ProfileService struct {
	profileRepo      repository.ProfileRepository
	statsRepo        repository.StatsRepository
	achievementRepo  repository.AchievementRepository // ← это репозиторий, а не сервис
	log              *logger.Logger
}

func NewProfileService(
	profileRepo repository.ProfileRepository,
	statsRepo repository.StatsRepository,
	achievementRepo repository.AchievementRepository, // ← репозиторий
	log *logger.Logger,
) *ProfileService {
	return &ProfileService{
		profileRepo:     profileRepo,
		statsRepo:       statsRepo,
		achievementRepo: achievementRepo,
		log:             log,
	}
}

// CreateProfile создаёт профиль и статистику для нового пользователя
func (s *ProfileService) CreateProfile(ctx context.Context, userID uuid.UUID) error {
	if err := s.profileRepo.Create(ctx, userID); err != nil {
		return err
	}
	if err := s.statsRepo.Create(ctx, userID); err != nil {
		return err
	}
	s.log.Info("Профиль создан", "user_id", userID)
	return nil
}

// GetFullProfile получение полного профиля (для владельца)
func (s *ProfileService) GetFullProfile(ctx context.Context, userID uuid.UUID) (*models.Profile, error) {
	profile, err := s.profileRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, ErrProfileNotFound
	}
	return profile, nil
}

// GetPublicProfile получение публичного профиля
func (s *ProfileService) GetPublicProfile(ctx context.Context, userID uuid.UUID) (*models.Profile, error) {
	profile, err := s.profileRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, ErrProfileNotFound
	}
	return profile, nil
}

// UpdateProfile обновление профиля
func (s *ProfileService) UpdateProfile(ctx context.Context, userID uuid.UUID, updates *dto.UpdateProfileRequest) error {
	exists, err := s.profileRepo.Exists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrProfileNotFound
	}

	// Маппинг DTO в модель репозитория
	modelUpdates := &models.UpdateProfileRequest{
		FirstName:           updates.FirstName,
		LastName:            updates.LastName,
		Bio:                 updates.Bio,
		Phone:               updates.Phone,
		Language:            updates.Language,
		Timezone:            updates.Timezone,
		NotificationsEnabled: updates.NotificationsEnabled,
	}

	return s.profileRepo.Update(ctx, userID, modelUpdates)
}

// UpdateAvatar обновление аватара
func (s *ProfileService) UpdateAvatar(ctx context.Context, userID uuid.UUID, avatarURL string) error {
	exists, err := s.profileRepo.Exists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrProfileNotFound
	}
	return s.profileRepo.UpdateAvatar(ctx, userID, avatarURL)
}