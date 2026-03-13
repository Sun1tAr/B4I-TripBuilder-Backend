package service

import (
	"context"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type AchievementService struct {
	achievementRepo repository.AchievementRepository
	log             *logger.Logger
}

func NewAchievementService(achievementRepo repository.AchievementRepository, log *logger.Logger) *AchievementService {
	return &AchievementService{
		achievementRepo: achievementRepo,
		log:             log,
	}
}

func (s *AchievementService) GetUserAchievements(ctx context.Context, userID uuid.UUID) ([]*models.Achievement, error) {
	return s.achievementRepo.GetByUserID(ctx, userID)
}

// CheckAndAward проверяет условия и выдаёт достижения
func (s *AchievementService) CheckAndAward(ctx context.Context, userID uuid.UUID, eventType string, value int) error {
	// Будет реализовано позже через Kafka consumer
	return nil
}