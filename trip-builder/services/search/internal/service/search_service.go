package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type SearchService struct {
	// Основные репозитории (ES)
	routeRepo repository.RouteRepository
	eventRepo repository.EventRepository
	userRepo  repository.UserRepository

	// Резервные репозитории (Redis)
	routeRedis repository.RouteRepository
	eventRedis repository.EventRepository
	userRedis  repository.UserRepository

	esAvailable *atomic.Bool
	cfg         *config.Config
	log         *logger.Logger
	httpClient  *http.Client

	// Флаг, что реиндексация Redis уже была выполнена при текущем падении ES
	redisReindexed atomic.Bool
}

func NewSearchService(
	routeES repository.RouteRepository,
	eventES repository.EventRepository,
	userES repository.UserRepository,
	routeRedis repository.RouteRepository,
	eventRedis repository.EventRepository,
	userRedis repository.UserRepository,
	esAvailable *atomic.Bool,
	cfg *config.Config,
	log *logger.Logger,
) *SearchService {
	return &SearchService{
		routeRepo:   routeES,
		eventRepo:   eventES,
		userRepo:    userES,
		routeRedis:  routeRedis,
		eventRedis:  eventRedis,
		userRedis:   userRedis,
		esAvailable: esAvailable,
		cfg:         cfg,
		log:         log,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *SearchService) getRouteRepo() repository.RouteRepository {
	if s.esAvailable.Load() && s.routeRepo != nil {
		return s.routeRepo
	}
	if s.routeRedis != nil {
		return s.routeRedis
	}
	s.log.Error("Нет доступного репозитория для маршрутов!")
	return nil
}

func (s *SearchService) getEventRepo() repository.EventRepository {
	if s.esAvailable.Load() && s.eventRepo != nil {
		return s.eventRepo
	}
	if s.eventRedis != nil {
		return s.eventRedis
	}
	s.log.Error("Нет доступного репозитория для событий!")
	return nil
}

func (s *SearchService) getUserRepo() repository.UserRepository {
	if s.esAvailable.Load() && s.userRepo != nil {
		return s.userRepo
	}
	if s.userRedis != nil {
		return s.userRedis
	}
	s.log.Error("Нет доступного репозитория для пользователей!")
	return nil
}

// ========== МАРШРУТЫ С FALLBACK ==========

func (s *SearchService) IndexRoute(ctx context.Context, doc *models.RouteDocument) error {
	repo := s.getRouteRepo()
	if repo == nil {
		return fmt.Errorf("no route repository available")
	}
	return repo.Index(ctx, doc)
}

func (s *SearchService) DeleteRoute(ctx context.Context, id string) error {
	repo := s.getRouteRepo()
	if repo == nil {
		return fmt.Errorf("no route repository available")
	}
	return repo.Delete(ctx, id)
}

func (s *SearchService) SearchRoutes(ctx context.Context, req *models.RouteSearchRequest) (*models.RouteSearchResponse, error) {
	// Если ES не считается доступным – сразу идём в Redis
	if !s.esAvailable.Load() {
		s.log.Debug("ES не доступен, используем Redis")
		if s.routeRedis == nil {
			return nil, fmt.Errorf("redis repository not available")
		}
		return s.routeRedis.Search(ctx, req)
	}

	// Пробуем ES с retry
	var lastErr error
	for attempt := 0; attempt < s.cfg.ESMaxRetries; attempt++ {
		if s.routeRepo == nil {
			break
		}
		resp, err := s.routeRepo.Search(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		s.log.Warn("ES запрос failed", "attempt", attempt+1, "error", err)
		time.Sleep(s.cfg.ESRetryDelay * time.Duration(attempt+1))
	}

	// ES умер – переключаемся на Redis
	s.log.Error("ES недоступен после retry, переключение на Redis", "error", lastErr)

	// Реиндексация Redis (только один раз при текущем падении ES)
	if !s.redisReindexed.Load() {
		s.log.Info("Запуск реиндексации Redis (при падении ES)")
		if err := s.FullReindexRedis(ctx); err != nil {
			s.log.Error("Ошибка реиндексации Redis", "error", err)
		} else {
			s.redisReindexed.Store(true)
		}
	}

	// Переключаем флаг
	s.esAvailable.Store(false)

	// Выполняем запрос в Redis
	if s.routeRedis == nil {
		return nil, fmt.Errorf("redis repository not available")
	}
	return s.routeRedis.Search(ctx, req)
}

// ========== СОБЫТИЯ С FALLBACK ==========

func (s *SearchService) IndexEvent(ctx context.Context, doc *models.EventDocument) error {
	repo := s.getEventRepo()
	if repo == nil {
		return fmt.Errorf("no event repository available")
	}
	return repo.Index(ctx, doc)
}

func (s *SearchService) DeleteEvent(ctx context.Context, id string) error {
	repo := s.getEventRepo()
	if repo == nil {
		return fmt.Errorf("no event repository available")
	}
	return repo.Delete(ctx, id)
}

func (s *SearchService) SearchEvents(ctx context.Context, req *models.EventSearchRequest) (*models.EventSearchResponse, error) {
	if !s.esAvailable.Load() {
		if s.eventRedis == nil {
			return nil, fmt.Errorf("redis repository not available")
		}
		return s.eventRedis.Search(ctx, req)
	}

	var lastErr error
	for attempt := 0; attempt < s.cfg.ESMaxRetries; attempt++ {
		if s.eventRepo == nil {
			break
		}
		resp, err := s.eventRepo.Search(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		time.Sleep(s.cfg.ESRetryDelay * time.Duration(attempt+1))
	}

	s.log.Error("ES недоступен для событий, переключение на Redis", "error", lastErr)

	if !s.redisReindexed.Load() {
		s.log.Info("Запуск реиндексации Redis (при падении ES для событий)")
		if err := s.FullReindexRedis(ctx); err != nil {
			s.log.Error("Ошибка реиндексации Redis", "error", err)
		} else {
			s.redisReindexed.Store(true)
		}
	}

	s.esAvailable.Store(false)

	if s.eventRedis == nil {
		return nil, fmt.Errorf("redis repository not available")
	}
	return s.eventRedis.Search(ctx, req)
}

// ========== ПОЛЬЗОВАТЕЛИ (ТОЛЬКО ES, НЕТ FALLBACK) ==========

func (s *SearchService) IndexUser(ctx context.Context, doc *models.UserDocument) error {
	repo := s.getUserRepo()
	if repo == nil {
		return fmt.Errorf("no user repository available")
	}
	return repo.Index(ctx, doc)
}

func (s *SearchService) DeleteUser(ctx context.Context, id string) error {
	repo := s.getUserRepo()
	if repo == nil {
		return fmt.Errorf("no user repository available")
	}
	return repo.Delete(ctx, id)
}

func (s *SearchService) SearchUsers(ctx context.Context, req *models.UserSearchRequest) (*models.UserSearchResponse, error) {
	// Поиск пользователей только через ES (приватные данные)
	if s.esAvailable.Load() && s.userRepo != nil {
		return s.userRepo.Search(ctx, req)
	}
	return nil, fmt.Errorf("user search is only available when Elasticsearch is active")
}

// ========== SUGGEST (ТОЛЬКО ES) ==========

func (s *SearchService) Suggest(ctx context.Context, req *models.SuggestRequest) (*models.SuggestResponse, error) {
	if !s.esAvailable.Load() {
		return &models.SuggestResponse{Suggestions: []models.Suggestion{}}, nil
	}
	if s.routeRepo == nil {
		return &models.SuggestResponse{Suggestions: []models.Suggestion{}}, nil
	}
	suggestions, err := s.routeRepo.Suggest(ctx, req.Query, req.Size)
	if err != nil {
		return nil, err
	}
	return &models.SuggestResponse{Suggestions: suggestions}, nil
}

// ========== РЕИНДЕКСАЦИЯ ==========

// FullReindexES полная переиндексация Elasticsearch (при восстановлении ES)
func (s *SearchService) FullReindexES(ctx context.Context) error {
	s.log.Info("Начало полной реиндексации Elasticsearch")

	routes, err := s.fetchAllRoutes(ctx)
	if err != nil {
		return err
	}
	if s.routeRepo != nil {
		if err := s.routeRepo.ReindexAll(ctx, routes); err != nil {
			return err
		}
	}

	events, err := s.fetchAllEvents(ctx)
	if err != nil {
		return err
	}
	if s.eventRepo != nil {
		if err := s.eventRepo.ReindexAll(ctx, events); err != nil {
			return err
		}
	}

	users, err := s.fetchAllUsers(ctx)
	if err != nil {
		return err
	}
	if s.userRepo != nil {
		if err := s.userRepo.ReindexAll(ctx, users); err != nil {
			return err
		}
	}

	s.log.Info("Полная реиндексация Elasticsearch завершена")
	return nil
}

// FullReindexRedis полная переиндексация Redis (при падении ES)
func (s *SearchService) FullReindexRedis(ctx context.Context) error {
	s.log.Info("Начало полной реиндексации Redis")

	routes, err := s.fetchAllRoutes(ctx)
	if err != nil {
		return err
	}
	if s.routeRedis != nil {
		if err := s.routeRedis.ReindexAll(ctx, routes); err != nil {
			return err
		}
	}

	events, err := s.fetchAllEvents(ctx)
	if err != nil {
		return err
	}
	if s.eventRedis != nil {
		if err := s.eventRedis.ReindexAll(ctx, events); err != nil {
			return err
		}
	}

	users, err := s.fetchAllUsers(ctx)
	if err != nil {
		return err
	}
	if s.userRedis != nil {
		if err := s.userRedis.ReindexAll(ctx, users); err != nil {
			return err
		}
	}

	s.log.Info("Полная реиндексация Redis завершена")
	return nil
}

// ========== FETCH DATA FROM SERVICES ==========

func (s *SearchService) fetchAllRoutes(ctx context.Context) ([]*models.RouteDocument, error) {
	url := fmt.Sprintf("%s/api/v1/routes?size=10000", s.cfg.RouteServiceURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response struct {
		Items []struct {
			ID          string    `json:"id"`
			Title       string    `json:"title"`
			Description string    `json:"description"`
			Country     string    `json:"country"`
			UserID      string    `json:"user_id"`
			IsPublic    bool      `json:"is_public"`
			TotalDays   int       `json:"total_days"`
			LikesCount  int       `json:"likes_count"`
			CreatedAt   time.Time `json:"created_at"`
			Tags        []struct {
				Name string `json:"name"`
			} `json:"tags"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	routes := make([]*models.RouteDocument, 0, len(response.Items))
	for _, item := range response.Items {
		tags := make([]string, len(item.Tags))
		for i, t := range item.Tags {
			tags[i] = t.Name
		}
		routes = append(routes, &models.RouteDocument{
			ID:          item.ID,
			Title:       item.Title,
			Description: item.Description,
			Country:     item.Country,
			Tags:        tags,
			TotalDays:   item.TotalDays,
			UserID:      item.UserID,
			LikesCount:  item.LikesCount,
			IsPublic:    item.IsPublic,
			CreatedAt:   item.CreatedAt,
		})
	}
	return routes, nil
}

func (s *SearchService) fetchAllEvents(ctx context.Context) ([]*models.EventDocument, error) {
	url := fmt.Sprintf("%s/api/v1/events/public?size=10000", s.cfg.EventServiceURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response struct {
		Items []struct {
			ID        string    `json:"id"`
			Title     string    `json:"title"`
			RouteID   string    `json:"route_id"`
			StartDate time.Time `json:"start_date"`
			Status    string    `json:"status"`
			IsPublic  bool      `json:"is_public"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	events := make([]*models.EventDocument, 0, len(response.Items))
	for _, item := range response.Items {
		events = append(events, &models.EventDocument{
			ID:        item.ID,
			Title:     item.Title,
			RouteID:   item.RouteID,
			StartDate: item.StartDate,
			Status:    item.Status,
			IsPublic:  item.IsPublic,
		})
	}
	return events, nil
}

func (s *SearchService) fetchAllUsers(ctx context.Context) ([]*models.UserDocument, error) {
	url := fmt.Sprintf("%s/api/v1/users?size=10000", s.cfg.UserServiceURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response struct {
		Items []struct {
			UserID    string `json:"user_id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Bio       string `json:"bio"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	users := make([]*models.UserDocument, 0, len(response.Items))
	for _, item := range response.Items {
		users = append(users, &models.UserDocument{
			ID:        item.UserID,
			FirstName: item.FirstName,
			LastName:  item.LastName,
			Bio:       item.Bio,
		})
	}
	return users, nil
}
