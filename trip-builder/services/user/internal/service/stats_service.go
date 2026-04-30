package service

import (
	"context"
	"errors"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

var ErrStatsNotFound = errors.New("stats not found")

type StatsResult struct {
	UserID           uuid.UUID
	RoutesCreated    int
	EventsDone       int
	CountriesVisited int
	PhotosUploaded   int
	Followers        int
	Following        int
}

type StatsService struct {
	statsRepo      repository.StatsRepository
	achievementSvc *AchievementService
	log            *logger.Logger
}

func NewStatsService(
	statsRepo repository.StatsRepository,
	achievementSvc *AchievementService,
	log *logger.Logger,
) *StatsService {
	return &StatsService{
		statsRepo:      statsRepo,
		achievementSvc: achievementSvc,
		log:            log,
	}
}

func (s *StatsService) GetUserStats(ctx context.Context, userID uuid.UUID) (*StatsResult, error) {
	s.log.Debug("Получение статистики пользователя", "user_id", userID)

	stats, err := s.statsRepo.GetByID(ctx, userID)
	if err != nil {
		s.log.Error("Ошибка получения статистики", "error", err, "user_id", userID)
		return nil, err
	}
	if stats == nil {
		s.log.Debug("Статистика не найдена, возвращаем пустую", "user_id", userID)
		return &StatsResult{UserID: userID}, nil
	}

	return &StatsResult{
		UserID:           stats.UserID,
		RoutesCreated:    stats.RoutesCreated,
		EventsDone:       stats.EventsDone,
		CountriesVisited: stats.CountriesVisited,
		PhotosUploaded:   stats.PhotosUploaded,
		Followers:        stats.Followers,
		Following:        stats.Following,
	}, nil
}

// IncrementRoutesCreated увеличивает счётчик созданных маршрутов
func (s *StatsService) IncrementRoutesCreated(ctx context.Context, userID uuid.UUID) error {
	s.log.Debug("Инкремент routes_created", "user_id", userID)

	if err := s.statsRepo.IncrementRoutesCreated(ctx, userID); err != nil {
		s.log.Error("Ошибка инкремента routes_created", "error", err, "user_id", userID)
		return err
	}

	// Проверяем достижения
	if s.achievementSvc != nil {
		if err := s.achievementSvc.CheckAndAward(ctx, userID, "routes_created", 0); err != nil {
			s.log.Error("Ошибка проверки достижений после routes_created", "error", err, "user_id", userID)
		}
	}

	return nil
}

// IncrementEventsDone увеличивает счётчик завершённых событий
func (s *StatsService) IncrementEventsDone(ctx context.Context, userID uuid.UUID) error {
	s.log.Debug("Инкремент events_done", "user_id", userID)

	if err := s.statsRepo.IncrementEventsDone(ctx, userID); err != nil {
		s.log.Error("Ошибка инкремента events_done", "error", err, "user_id", userID)
		return err
	}

	// Проверяем достижения
	if s.achievementSvc != nil {
		if err := s.achievementSvc.CheckAndAward(ctx, userID, "events_done", 0); err != nil {
			s.log.Error("Ошибка проверки достижений после events_done", "error", err, "user_id", userID)
		}
	}

	return nil
}

// IncrementCountriesVisited увеличивает счётчик посещённых стран
func (s *StatsService) IncrementCountriesVisited(ctx context.Context, userID uuid.UUID, count int) error {
	s.log.Debug("Инкремент countries_visited", "user_id", userID, "count", count)

	if err := s.statsRepo.IncrementCountriesVisited(ctx, userID, count); err != nil {
		s.log.Error("Ошибка инкремента countries_visited", "error", err, "user_id", userID)
		return err
	}

	// Проверяем достижения
	if s.achievementSvc != nil {
		if err := s.achievementSvc.CheckAndAward(ctx, userID, "countries_visited", 0); err != nil {
			s.log.Error("Ошибка проверки достижений после countries_visited", "error", err, "user_id", userID)
		}
	}

	return nil
}

// IncrementPhotosUploaded увеличивает счётчик загруженных фото
func (s *StatsService) IncrementPhotosUploaded(ctx context.Context, userID uuid.UUID) error {
	s.log.Debug("Инкремент photos_uploaded", "user_id", userID)

	if err := s.statsRepo.IncrementPhotosUploaded(ctx, userID); err != nil {
		s.log.Error("Ошибка инкремента photos_uploaded", "error", err, "user_id", userID)
		return err
	}

	// Проверяем достижения
	if s.achievementSvc != nil {
		if err := s.achievementSvc.CheckAndAward(ctx, userID, "photos_uploaded", 0); err != nil {
			s.log.Error("Ошибка проверки достижений после photos_uploaded", "error", err, "user_id", userID)
		}
	}

	return nil
}

// IncrementFollowers увеличивает счётчик подписчиков
func (s *StatsService) IncrementFollowers(ctx context.Context, userID uuid.UUID) error {
	s.log.Debug("Инкремент followers", "user_id", userID)

	if err := s.statsRepo.IncrementFollowers(ctx, userID); err != nil {
		s.log.Error("Ошибка инкремента followers", "error", err, "user_id", userID)
		return err
	}

	// Проверяем достижения
	if s.achievementSvc != nil {
		if err := s.achievementSvc.CheckAndAward(ctx, userID, "followers", 0); err != nil {
			s.log.Error("Ошибка проверки достижений после followers", "error", err, "user_id", userID)
		}
	}

	return nil
}

// DecrementFollowers уменьшает счётчик подписчиков
func (s *StatsService) DecrementFollowers(ctx context.Context, userID uuid.UUID) error {
	s.log.Debug("Декремент followers", "user_id", userID)

	if err := s.statsRepo.DecrementFollowers(ctx, userID); err != nil {
		s.log.Error("Ошибка декремента followers", "error", err, "user_id", userID)
		return err
	}

	return nil
}

// IncrementFollowing увеличивает счётчик подписок
func (s *StatsService) IncrementFollowing(ctx context.Context, userID uuid.UUID) error {
	s.log.Debug("Инкремент following", "user_id", userID)

	if err := s.statsRepo.IncrementFollowing(ctx, userID); err != nil {
		s.log.Error("Ошибка инкремента following", "error", err, "user_id", userID)
		return err
	}

	// Проверяем достижения
	if s.achievementSvc != nil {
		if err := s.achievementSvc.CheckAndAward(ctx, userID, "following", 0); err != nil {
			s.log.Error("Ошибка проверки достижений после following", "error", err, "user_id", userID)
		}
	}

	return nil
}

// DecrementFollowing уменьшает счётчик подписок
func (s *StatsService) DecrementFollowing(ctx context.Context, userID uuid.UUID) error {
	s.log.Debug("Декремент following", "user_id", userID)

	if err := s.statsRepo.DecrementFollowing(ctx, userID); err != nil {
		s.log.Error("Ошибка декремента following", "error", err, "user_id", userID)
		return err
	}

	return nil
}