package service

import (
	"context"
	"errors"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/kafka"
	userdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/user"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

var (
	ErrProfileNotFound = errors.New("profile not found")
)

type ProfileService struct {
	profileRepo     repository.ProfileRepository
	statsRepo       repository.StatsRepository
	achievementRepo repository.AchievementRepository
	userProducer    *kafka.UserProducer
	log             *logger.Logger
}

func NewProfileService(
	profileRepo repository.ProfileRepository,
	statsRepo repository.StatsRepository,
	achievementRepo repository.AchievementRepository,
	userProducer *kafka.UserProducer,
	log *logger.Logger,
) *ProfileService {
	return &ProfileService{
		profileRepo:     profileRepo,
		statsRepo:       statsRepo,
		achievementRepo: achievementRepo,
		userProducer:    userProducer,
		log:             log,
	}
}

// CreateProfile создаёт профиль и статистику для нового пользователя (идемпотентно)
func (s *ProfileService) CreateProfile(ctx context.Context, userID uuid.UUID, firstName, lastName string) error {
	// Проверяем, существует ли уже профиль
	exists, err := s.profileRepo.Exists(ctx, userID)
	if err != nil {
		return err
	}
	if exists {
		s.log.Debug("Профиль уже существует, пропускаем создание", "user_id", userID)
		return nil
	}

	if err := s.profileRepo.Create(ctx, userID, firstName, lastName); err != nil {
		s.log.Error("Ошибка создания профиля", "error", err, "user_id", userID)
		return err
	}
	if err := s.statsRepo.Create(ctx, userID); err != nil {
		s.log.Error("Ошибка создания статистики", "error", err, "user_id", userID)
		return err
	}
	s.log.Info("Профиль создан", "user_id", userID, "first_name", firstName, "last_name", lastName)
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
func (s *ProfileService) UpdateProfile(ctx context.Context, userID uuid.UUID, updates *userdto.UpdateProfileRequest) error {
	s.log.Debug("UpdateProfile: обновление профиля", "user_id", userID)

	exists, err := s.profileRepo.Exists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrProfileNotFound
	}

	// Получаем старые данные для сравнения
	oldProfile, err := s.profileRepo.GetByID(ctx, userID)
	if err != nil {
		s.log.Error("UpdateProfile: ошибка получения старого профиля", "error", err)
		// продолжаем, но без публикации
	}

	modelUpdates := &models.UpdateProfileRequest{
		FirstName:            updates.FirstName,
		LastName:             updates.LastName,
		Bio:                  updates.Bio,
		Phone:                updates.Phone,
		Language:             updates.Language,
		Timezone:             updates.Timezone,
		NotificationsEnabled: updates.NotificationsEnabled,
	}

	if err := s.profileRepo.Update(ctx, userID, modelUpdates); err != nil {
		return err
	}

	// Публикуем событие об обновлении, если есть изменения в важных полях
	if s.userProducer != nil && oldProfile != nil {
		firstName := oldProfile.FirstName
		if updates.FirstName != nil {
			firstName = *updates.FirstName
		}
		lastName := oldProfile.LastName
		if updates.LastName != nil {
			lastName = *updates.LastName
		}
		avatarURL := oldProfile.AvatarURL
		// avatarURL может обновиться отдельно через UploadAvatar

		if err := s.userProducer.PublishUserUpdated(ctx, userID, firstName, lastName, avatarURL); err != nil {
			s.log.Error("UpdateProfile: ошибка публикации user.updated", "error", err)
			// не фатально
		}
	}

	s.log.Info("UpdateProfile: профиль обновлён", "user_id", userID)
	return nil
}

// UpdateAvatar обновление аватара
func (s *ProfileService) UpdateAvatar(ctx context.Context, userID uuid.UUID, avatarURL string) error {
	s.log.Debug("UpdateAvatar: обновление аватара", "user_id", userID)

	exists, err := s.profileRepo.Exists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrProfileNotFound
	}

	// Получаем старые данные для публикации
	oldProfile, err := s.profileRepo.GetByID(ctx, userID)
	if err != nil {
		s.log.Error("UpdateAvatar: ошибка получения старого профиля", "error", err)
	}

	if err := s.profileRepo.UpdateAvatar(ctx, userID, avatarURL); err != nil {
		return err
	}

	// Публикуем событие об обновлении
	if s.userProducer != nil && oldProfile != nil {
		if err := s.userProducer.PublishUserUpdated(ctx, userID, oldProfile.FirstName, oldProfile.LastName, avatarURL); err != nil {
			s.log.Error("UpdateAvatar: ошибка публикации user.updated", "error", err)
		}
	}

	s.log.Info("UpdateAvatar: аватар обновлён", "user_id", userID, "avatar_url", avatarURL)
	return nil
}
