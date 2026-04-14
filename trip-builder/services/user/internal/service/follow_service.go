package service

import (
	"context"
	"errors"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

var (
	ErrAlreadyFollowing = errors.New("already following")
	ErrNotFollowing     = errors.New("not following")
)

type FollowService struct {
	followRepo     repository.FollowRepository
	profileRepo    repository.ProfileRepository
	statsService   StatsService
	achievementSvc AchievementService
	log            *logger.Logger
}

func NewFollowService(
	followRepo repository.FollowRepository,
	profileRepo repository.ProfileRepository,
	statsService *StatsService,
	achievementSvc *AchievementService,
	log *logger.Logger,
) *FollowService {
	return &FollowService{
		followRepo:     followRepo,
		profileRepo:    profileRepo,
		statsService:   *statsService,
		achievementSvc: *achievementSvc,
		log:            log,
	}
}

func (s *FollowService) Follow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	// Проверяем, что followee существует
	exists, err := s.profileRepo.Exists(ctx, followeeID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrProfileNotFound
	}

	// Проверяем, не подписан ли уже
	isFollowing, err := s.followRepo.IsFollowing(ctx, followerID, followeeID)
	if err != nil {
		return err
	}
	if isFollowing {
		return ErrAlreadyFollowing
	}

	// Подписываемся
	if err := s.followRepo.Follow(ctx, followerID, followeeID); err != nil {
		return err
	}

	// Увеличиваем followers у того, на кого подписались
	if err := s.statsService.IncrementFollowers(ctx, followeeID); err != nil {
		s.log.Error("Ошибка обновления followers", "error", err, "user_id", followeeID)
	}

	// Проверяем достижения для обоих пользователей
	go s.achievementSvc.CheckAndAward(context.Background(), followerID, "following", 0)
	go s.achievementSvc.CheckAndAward(context.Background(), followeeID, "followers", 0)

	s.log.Info("Пользователь подписался",
		"follower_id", followerID, "followee_id", followeeID)
	return nil
}

func (s *FollowService) Unfollow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	// Проверяем, подписан ли
	isFollowing, err := s.followRepo.IsFollowing(ctx, followerID, followeeID)
	if err != nil {
		return err
	}
	if !isFollowing {
		return ErrNotFollowing
	}

	if err := s.followRepo.Unfollow(ctx, followerID, followeeID); err != nil {
		return err
	}

	// Обновляем статистику
	if err := s.statsService.DecrementFollowing(ctx, followerID); err != nil {
		s.log.Error("Ошибка обновления following", "error", err, "user_id", followerID)
	}

	if err := s.statsService.DecrementFollowers(ctx, followeeID); err != nil {
		s.log.Error("Ошибка обновления followers", "error", err, "user_id", followeeID)
	}

	s.log.Info("Пользователь отписался",
		"follower_id", followerID, "followee_id", followeeID)
	return nil
}

func (s *FollowService) GetFollowers(ctx context.Context, userID uuid.UUID, limit, offset int) ([]uuid.UUID, int, error) {
	followers, err := s.followRepo.GetFollowers(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.followRepo.CountFollowers(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return followers, total, nil
}

func (s *FollowService) GetFollowing(ctx context.Context, userID uuid.UUID, limit, offset int) ([]uuid.UUID, int, error) {
	following, err := s.followRepo.GetFollowing(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.followRepo.CountFollowing(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return following, total, nil
}
