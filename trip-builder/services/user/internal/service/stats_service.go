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
	achievementSvc *AchievementService // добавляем зависимость
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
	stats, err := s.statsRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if stats == nil {
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

// Пример метода, который будет вызываться при получении событий
func (s *StatsService) IncrementRoutesCreated(ctx context.Context, userID uuid.UUID) error {
	if err := s.statsRepo.IncrementRoutesCreated(ctx, userID); err != nil {
		return err
	}
	// Проверяем достижения
	if s.achievementSvc != nil {
		_ = s.achievementSvc.CheckAndAward(ctx, userID, "routes_created", 0)
	}
	return nil
}