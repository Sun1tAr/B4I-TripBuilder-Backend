package service

import (
	"context"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"errors"
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
	statsRepo repository.StatsRepository
	log       *logger.Logger
}

func NewStatsService(statsRepo repository.StatsRepository, log *logger.Logger) *StatsService {
	return &StatsService{
		statsRepo: statsRepo,
		log:       log,
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