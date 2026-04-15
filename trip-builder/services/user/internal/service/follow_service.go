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

// Follow подписка на пользователя
func (s *FollowService) Follow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	s.log.Debug("Follow: начало", "follower_id", followerID, "followee_id", followeeID)

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

	// ========== ОБНОВЛЯЕМ СТАТИСТИКУ ==========

	// Увеличиваем following у подписчика
	if err := s.statsService.IncrementFollowing(ctx, followerID); err != nil {
		s.log.Error("Follow: ошибка обновления following", "error", err, "user_id", followerID)
	} else {
		s.log.Debug("Follow: following увеличен", "user_id", followerID)
	}

	// Увеличиваем followers у того, на кого подписались
	if err := s.statsService.IncrementFollowers(ctx, followeeID); err != nil {
		s.log.Error("Follow: ошибка обновления followers", "error", err, "user_id", followeeID)
	} else {
		s.log.Debug("Follow: followers увеличен", "user_id", followeeID)
	}

	// ========== ПРОВЕРЯЕМ ДОСТИЖЕНИЯ ==========
	go s.achievementSvc.CheckAndAward(context.Background(), followerID, "following", 0)
	go s.achievementSvc.CheckAndAward(context.Background(), followeeID, "followers", 0)

	s.log.Info("Подписка оформлена", "follower_id", followerID, "followee_id", followeeID)
	return nil
}

// Unfollow отписка от пользователя
func (s *FollowService) Unfollow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	s.log.Debug("Unfollow: начало", "follower_id", followerID, "followee_id", followeeID)

	// Проверяем, подписан ли
	isFollowing, err := s.followRepo.IsFollowing(ctx, followerID, followeeID)
	if err != nil {
		return err
	}
	if !isFollowing {
		return ErrNotFollowing
	}

	// Отписываемся
	if err := s.followRepo.Unfollow(ctx, followerID, followeeID); err != nil {
		return err
	}

	// ========== ОБНОВЛЯЕМ СТАТИСТИКУ ==========

	// Уменьшаем following у подписчика
	if err := s.statsService.DecrementFollowing(ctx, followerID); err != nil {
		s.log.Error("Unfollow: ошибка обновления following", "error", err, "user_id", followerID)
	} else {
		s.log.Debug("Unfollow: following уменьшен", "user_id", followerID)
	}

	// Уменьшаем followers у того, от кого отписались
	if err := s.statsService.DecrementFollowers(ctx, followeeID); err != nil {
		s.log.Error("Unfollow: ошибка обновления followers", "error", err, "user_id", followeeID)
	} else {
		s.log.Debug("Unfollow: followers уменьшен", "user_id", followeeID)
	}

	s.log.Info("Отписка оформлена", "follower_id", followerID, "followee_id", followeeID)
	return nil
}

// GetFollowers возвращает список подписчиков
func (s *FollowService) GetFollowers(ctx context.Context, userID uuid.UUID, limit, offset int) ([]uuid.UUID, int, error) {
	s.log.Debug("GetFollowers: начало", "user_id", userID)

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

// GetFollowing возвращает список подписок
func (s *FollowService) GetFollowing(ctx context.Context, userID uuid.UUID, limit, offset int) ([]uuid.UUID, int, error) {
	s.log.Debug("GetFollowing: начало", "user_id", userID)

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
