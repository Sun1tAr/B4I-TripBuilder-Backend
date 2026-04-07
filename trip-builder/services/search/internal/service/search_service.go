package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type SearchService struct {
	routeRepo  repository.RouteRepository
	eventRepo  repository.EventRepository
	userRepo   repository.UserRepository
	cfg        *config.Config
	log        *logger.Logger
	httpClient *http.Client
}

func NewSearchService(
	routeRepo repository.RouteRepository,
	eventRepo repository.EventRepository,
	userRepo repository.UserRepository,
	cfg *config.Config,
	log *logger.Logger,
) *SearchService {
	return &SearchService{
		routeRepo:  routeRepo,
		eventRepo:  eventRepo,
		userRepo:   userRepo,
		cfg:        cfg,
		log:        log,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Index маршрутов
func (s *SearchService) IndexRoute(ctx context.Context, doc *models.RouteDocument) error {
	return s.routeRepo.IndexDocument(ctx, doc)
}

func (s *SearchService) DeleteRoute(ctx context.Context, routeID string) error {
	return s.routeRepo.DeleteDocument(ctx, routeID)
}

func (s *SearchService) SearchRoutes(ctx context.Context, req *models.RouteSearchRequest) (*models.RouteSearchResponse, error) {
	return s.routeRepo.Search(ctx, req)
}

// Index событий
func (s *SearchService) IndexEvent(ctx context.Context, doc *models.EventDocument) error {
	return s.eventRepo.IndexDocument(ctx, doc)
}

func (s *SearchService) DeleteEvent(ctx context.Context, eventID string) error {
	return s.eventRepo.DeleteDocument(ctx, eventID)
}

func (s *SearchService) SearchEvents(ctx context.Context, req *models.EventSearchRequest) (*models.EventSearchResponse, error) {
	return s.eventRepo.Search(ctx, req)
}

// Index пользователей
func (s *SearchService) IndexUser(ctx context.Context, doc *models.UserDocument) error {
	return s.userRepo.IndexDocument(ctx, doc)
}

func (s *SearchService) DeleteUser(ctx context.Context, userID string) error {
	return s.userRepo.DeleteDocument(ctx, userID)
}

func (s *SearchService) SearchUsers(ctx context.Context, req *models.UserSearchRequest) (*models.UserSearchResponse, error) {
	return s.userRepo.Search(ctx, req)
}

// Автодополнение
func (s *SearchService) Suggest(ctx context.Context, query string, suggestType string, size int) (*models.SuggestResponse, error) {
	s.log.Debug("Автодополнение", "query", query, "type", suggestType, "size", size)

	allSuggestions := []models.Suggestion{}

	if suggestType == "" || suggestType == "routes" {
		routeSugs, err := s.routeRepo.Suggest(ctx, query, size)
		if err != nil {
			s.log.Error("Ошибка получения подсказок маршрутов", "error", err)
		} else {
			allSuggestions = append(allSuggestions, routeSugs...)
		}
	}

	if suggestType == "" || suggestType == "events" {
		eventSugs, err := s.eventRepo.Suggest(ctx, query, size)
		if err != nil {
			s.log.Error("Ошибка получения подсказок событий", "error", err)
		} else {
			allSuggestions = append(allSuggestions, eventSugs...)
		}
	}

	if suggestType == "" || suggestType == "users" {
		userSugs, err := s.userRepo.Suggest(ctx, query, size)
		if err != nil {
			s.log.Error("Ошибка получения подсказок пользователей", "error", err)
		} else {
			allSuggestions = append(allSuggestions, userSugs...)
		}
	}

	// Ограничиваем количество
	if len(allSuggestions) > size {
		allSuggestions = allSuggestions[:size]
	}

	return &models.SuggestResponse{
		Suggestions: allSuggestions,
	}, nil
}

// Полная переиндексация
func (s *SearchService) FullReindex(ctx context.Context) error {
	s.log.Info("Начало полной переиндексации")

	// 1. Переиндексация маршрутов
	if err := s.reindexRoutes(ctx); err != nil {
		s.log.Error("Ошибка переиндексации маршрутов", "error", err)
	}

	// 2. Переиндексация событий
	if err := s.reindexEvents(ctx); err != nil {
		s.log.Error("Ошибка переиндексации событий", "error", err)
	}

	// 3. Переиндексация пользователей
	if err := s.reindexUsers(ctx); err != nil {
		s.log.Error("Ошибка переиндексации пользователей", "error", err)
	}

	s.log.Info("Полная переиндексация завершена")
	return nil
}

func (s *SearchService) reindexRoutes(ctx context.Context) error {
	// Загружаем все маршруты из Route Service
	url := fmt.Sprintf("%s/api/v1/routes?size=10000", s.cfg.RouteServiceURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch routes: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var routesResponse struct {
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

	if err := json.Unmarshal(body, &routesResponse); err != nil {
		return err
	}

	docs := make([]*models.RouteDocument, 0, len(routesResponse.Items))
	for _, item := range routesResponse.Items {
		if !item.IsPublic {
			continue
		}
		tagNames := make([]string, len(item.Tags))
		for i, tag := range item.Tags {
			tagNames[i] = tag.Name
		}

		docs = append(docs, &models.RouteDocument{
			ID:          item.ID,
			Title:       item.Title,
			Description: item.Description,
			Country:     item.Country,
			Tags:        tagNames,
			TotalDays:   item.TotalDays,
			UserID:      item.UserID,
			LikesCount:  item.LikesCount,
			CreatedAt:   item.CreatedAt,
			Suggest:     append([]string{item.Title, item.Country}, tagNames...),
		})
	}

	return s.routeRepo.ReindexAll(ctx, docs)
}

func (s *SearchService) reindexEvents(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/events/public?size=10000", s.cfg.EventServiceURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch events: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var eventsResponse struct {
		Items []struct {
			ID        string    `json:"id"`
			Title     string    `json:"title"`
			RouteID   string    `json:"route_id"`
			StartDate time.Time `json:"start_date"`
			Status    string    `json:"status"`
			IsPublic  bool      `json:"is_public"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &eventsResponse); err != nil {
		return err
	}

	docs := make([]*models.EventDocument, 0, len(eventsResponse.Items))
	for _, item := range eventsResponse.Items {
		if !item.IsPublic {
			continue
		}
		docs = append(docs, &models.EventDocument{
			ID:        item.ID,
			Title:     item.Title,
			RouteID:   item.RouteID,
			StartDate: item.StartDate,
			Status:    item.Status,
			IsPublic:  item.IsPublic,
			Suggest:   []string{item.Title},
		})
	}

	return s.eventRepo.ReindexAll(ctx, docs)
}

func (s *SearchService) reindexUsers(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/users?size=10000", s.cfg.UserServiceURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch users: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var usersResponse struct {
		Items []struct {
			UserID    string `json:"user_id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Bio       string `json:"bio"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &usersResponse); err != nil {
		return err
	}

	docs := make([]*models.UserDocument, 0, len(usersResponse.Items))
	for _, item := range usersResponse.Items {
		docs = append(docs, &models.UserDocument{
			ID:        item.UserID,
			FirstName: item.FirstName,
			LastName:  item.LastName,
			Bio:       item.Bio,
			Suggest:   []string{item.FirstName, item.LastName},
		})
	}

	return s.userRepo.ReindexAll(ctx, docs)
}

// SyncFullReindex запускает периодическую переиндексацию по крону
func (s *SearchService) StartPeriodicReindex(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		s.log.Info("Периодическая переиндексация отключена", "interval", interval)
		return
	}

	s.log.Info("Запуск периодической переиндексации", "interval", interval)

	// Первый запуск через interval, не сразу
	time.Sleep(interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("Остановка периодической переиндексации")
			return
		case <-ticker.C:
			s.log.Info("Запуск плановой переиндексации")
			if err := s.FullReindex(ctx); err != nil {
				s.log.Error("Ошибка плановой переиндексации", "error", err)
			}
		}
	}
}
