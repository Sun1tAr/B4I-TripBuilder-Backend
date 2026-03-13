package service

import (
	"context"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type AchievementService struct {
	achievementRepo repository.AchievementRepository
	statsRepo       repository.StatsRepository
	log             *logger.Logger
}

func NewAchievementService(
	achievementRepo repository.AchievementRepository,
	statsRepo repository.StatsRepository,
	log *logger.Logger,
) *AchievementService {
	return &AchievementService{
		achievementRepo: achievementRepo,
		statsRepo:       statsRepo,
		log:             log,
	}
}

func (s *AchievementService) GetUserAchievements(ctx context.Context, userID uuid.UUID) ([]*models.Achievement, error) {
	return s.achievementRepo.GetByUserID(ctx, userID)
}

// CheckAndAward проверяет условия и выдаёт достижения при изменении статистики
func (s *AchievementService) CheckAndAward(ctx context.Context, userID uuid.UUID, eventType string, newValue int) error {
	// Получаем текущую статистику
	stats, err := s.statsRepo.GetByID(ctx, userID)
	if err != nil || stats == nil {
		return err
	}

	var achievementsToCheck []func() (string, string, bool) // каждая функция возвращает type, title, условие выполнено

	switch eventType {
	case "routes_created":
		achievementsToCheck = []func() (string, string, bool){
			func() (string, string, bool) { return "first_route", "Первое путешествие", stats.RoutesCreated >= 1 },
			func() (string, string, bool) { return "ten_routes", "10 маршрутов", stats.RoutesCreated >= 10 },
		}
	case "events_done":
		achievementsToCheck = []func() (string, string, bool){
			func() (string, string, bool) { return "first_event", "Первое событие", stats.EventsDone >= 1 },
			func() (string, string, bool) { return "ten_events", "10 событий", stats.EventsDone >= 10 },
		}
	case "countries_visited":
		achievementsToCheck = []func() (string, string, bool){
			func() (string, string, bool) { return "first_country", "Первая страна", stats.CountriesVisited >= 1 },
			func() (string, string, bool) { return "ten_countries", "10 стран", stats.CountriesVisited >= 10 },
		}
	case "photos_uploaded":
		achievementsToCheck = []func() (string, string, bool){
			func() (string, string, bool) { return "first_photo", "Первое фото", stats.PhotosUploaded >= 1 },
			func() (string, string, bool) { return "hundred_photos", "100 фото", stats.PhotosUploaded >= 100 },
		}
	case "followers":
		achievementsToCheck = []func() (string, string, bool){
			func() (string, string, bool) { return "first_follower", "Первый подписчик", stats.Followers >= 1 },
		}
	case "following":
		achievementsToCheck = []func() (string, string, bool){
			func() (string, string, bool) { return "first_following", "Первая подписка", stats.Following >= 1 },
		}
	default:
		return nil
	}

	for _, check := range achievementsToCheck {
		achType, title, achieved := check()
		if achieved {
			// Проверяем, не выдано ли уже
			exists, err := s.achievementRepo.HasAchievement(ctx, userID, achType)
			if err != nil {
				s.log.Error("Ошибка проверки достижения", "error", err, "user_id", userID, "type", achType)
				continue
			}
			if !exists {
				desc := fmt.Sprintf("Достижение получено за %s", title)
				if err := s.achievementRepo.Create(ctx, userID, achType, title, desc); err != nil {
					s.log.Error("Ошибка сохранения достижения", "error", err, "user_id", userID, "type", achType)
				} else {
					s.log.Info("Достижение выдано", "user_id", userID, "type", achType, "title", title)
					// TODO: опубликовать событие user.achievement_earned
				}
			}
		}
	}

	return nil
}