package service

import (
	"context"
	"errors"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

var (
	ErrRouteNotFound      = errors.New("route not found")
	ErrNotOwner           = errors.New("user is not the owner of this route")
	ErrInvalidWaypoints   = errors.New("route must have at least one waypoint")
	ErrTitleTooShort      = errors.New("title must be at least 3 characters")
	ErrInvalidCoordinates = errors.New("invalid coordinates")
)

type RouteService struct {
	routeRepo repository.RouteRepository
	tagRepo   repository.TagRepository
	likeRepo  repository.LikeRepository
	producer  *kafka.RouteProducer
	log       *logger.Logger
}

func NewRouteService(
	routeRepo repository.RouteRepository,
	tagRepo repository.TagRepository,
	likeRepo repository.LikeRepository,
	producer *kafka.RouteProducer,
	log *logger.Logger,
) *RouteService {
	return &RouteService{
		routeRepo: routeRepo,
		tagRepo:   tagRepo,
		likeRepo:  likeRepo,
		producer:  producer,
		log:       log,
	}
}

// CreateRoute создаёт новый маршрут
func (s *RouteService) CreateRoute(ctx context.Context, userID uuid.UUID, req *models.CreateRouteRequest) (*models.RouteWithDetails, error) {
	s.log.Debug("Создание маршрута", "user_id", userID, "title", req.Title)

	// Валидация
	if len(req.Title) < 3 {
		return nil, ErrTitleTooShort
	}
	if len(req.Waypoints) == 0 {
		return nil, ErrInvalidWaypoints
	}

	// Создаём маршрут
	routeID := uuid.New()
	now := time.Now()

	// Рассчитываем общую длительность
	totalDays, totalHours := s.calculateTotalDuration(req.Waypoints)

	route := &models.Route{
		ID:          routeID,
		Title:       req.Title,
		Description: req.Description,
		Country:     req.Country,
		ImageURL:    req.ImageURL,
		UserID:      userID,
		IsPublic:    req.IsPublic,
		TotalDays:   totalDays,
		TotalHours:  totalHours,
		LikesCount:  0,
		EventsCount: 0,
		CreatedAt:   now,
	}

	// Создаём точки маршрута
	waypoints := make([]models.Waypoint, 0, len(req.Waypoints))
	for _, wpReq := range req.Waypoints {
		// Валидация координат
		if wpReq.Latitude < -90 || wpReq.Latitude > 90 || wpReq.Longitude < -180 || wpReq.Longitude > 180 {
			return nil, ErrInvalidCoordinates
		}

		waypoints = append(waypoints, models.Waypoint{
			ID:            uuid.New(),
			RouteID:       routeID,
			City:          wpReq.City,
			Description:   wpReq.Description,
			Latitude:      wpReq.Latitude,
			Longitude:     wpReq.Longitude,
			DurationDays:  wpReq.DurationDays,
			DurationHours: wpReq.DurationHours,
			OrderIndex:    wpReq.OrderIndex,
			CreatedAt:     now,
		})
	}

	// Сохраняем в БД
	if err := s.routeRepo.Create(ctx, route, waypoints); err != nil {
		s.log.Error("Ошибка создания маршрута в БД", "error", err, "user_id", userID)
		return nil, err
	}

	// Обрабатываем теги
	var tags []models.Tag
	if len(req.Tags) > 0 {
		tagIDs := make([]uuid.UUID, 0, len(req.Tags))

		for _, tagName := range req.Tags {
			tag, err := s.tagRepo.GetOrCreate(ctx, tagName)
			if err != nil {
				s.log.Error("Ошибка создания/получения тега", "error", err, "tag", tagName)
				continue
			}
			tagIDs = append(tagIDs, tag.ID)
			tags = append(tags, *tag)
		}

		if len(tagIDs) > 0 {
			if err := s.tagRepo.AttachToRoute(ctx, routeID, tagIDs); err != nil {
				s.log.Error("Ошибка привязки тегов к маршруту", "error", err, "route_id", routeID)
				// Не фатально, продолжаем
			}
		}
	}

	// Публикуем событие
	if err := s.producer.PublishRouteCreated(ctx, routeID, userID, route.Title, route.Country, route.IsPublic, route.TotalDays, route.TotalHours); err != nil {
		s.log.Error("Ошибка публикации события route.created", "error", err, "route_id", routeID)
		// Не фатально, продолжаем
	}

	s.log.Info("Маршрут успешно создан", "route_id", routeID, "user_id", userID)

	return &models.RouteWithDetails{
		Route:     *route,
		Waypoints: waypoints,
		Tags:      tags,
	}, nil
}

// GetRouteByID получает маршрут по ID
func (s *RouteService) GetRouteByID(ctx context.Context, routeID uuid.UUID, currentUserID *uuid.UUID) (*models.RouteWithDetails, error) {
	s.log.Debug("Получение маршрута", "route_id", routeID)

	route, waypoints, err := s.routeRepo.GetByIDWithDetails(ctx, routeID)
	if err != nil {
		s.log.Error("Ошибка получения маршрута", "error", err, "route_id", routeID)
		return nil, err
	}
	if route == nil {
		return nil, ErrRouteNotFound
	}

	// Проверяем доступность (публичный или владелец)
	if !route.IsPublic && (currentUserID == nil || *currentUserID != route.UserID) {
		s.log.Warn("Попытка доступа к приватному маршруту", "route_id", routeID, "user_id", currentUserID)
		return nil, ErrRouteNotFound
	}

	// Получаем теги
	tags, err := s.tagRepo.GetByRouteID(ctx, routeID)
	if err != nil {
		s.log.Error("Ошибка получения тегов маршрута", "error", err, "route_id", routeID)
		tags = []models.Tag{} // не фатально
	}

	// Проверяем, лайкнул ли текущий пользователь
	isLiked := false
	if currentUserID != nil {
		isLiked, err = s.likeRepo.IsLiked(ctx, *currentUserID, routeID)
		if err != nil {
			s.log.Error("Ошибка проверки лайка", "error", err, "route_id", routeID)
			isLiked = false
		}
	}

	return &models.RouteWithDetails{
		Route:     *route,
		Waypoints: waypoints,
		Tags:      tags,
		IsLiked:   isLiked,
	}, nil
}

// UpdateRoute обновляет маршрут
func (s *RouteService) UpdateRoute(ctx context.Context, routeID, userID uuid.UUID, req *models.UpdateRouteRequest) (*models.RouteWithDetails, error) {
	s.log.Debug("Обновление маршрута", "route_id", routeID, "user_id", userID)

	// Проверяем права
	isOwner, err := s.routeRepo.IsOwner(ctx, routeID, userID)
	if err != nil {
		s.log.Error("Ошибка проверки владельца", "error", err, "route_id", routeID)
		return nil, err
	}
	if !isOwner {
		return nil, ErrNotOwner
	}

	// Получаем существующий маршрут
	existingRoute, existingWaypoints, err := s.routeRepo.GetByIDWithDetails(ctx, routeID)
	if err != nil || existingRoute == nil {
		return nil, ErrRouteNotFound
	}

	// Обновляем поля маршрута
	if req.Title != nil {
		if len(*req.Title) < 3 {
			return nil, ErrTitleTooShort
		}
		existingRoute.Title = *req.Title
	}
	if req.Description != nil {
		existingRoute.Description = *req.Description
	}
	if req.Country != nil {
		existingRoute.Country = *req.Country
	}
	if req.ImageURL != nil {
		existingRoute.ImageURL = *req.ImageURL
	}
	if req.IsPublic != nil {
		existingRoute.IsPublic = *req.IsPublic
	}
	existingRoute.UpdatedAt = timePtr(time.Now())

	// Обновляем точки маршрута, если переданы
	var waypoints []models.Waypoint
	if req.Waypoints != nil {
		if len(req.Waypoints) == 0 {
			return nil, ErrInvalidWaypoints
		}

		waypoints = make([]models.Waypoint, 0, len(req.Waypoints))
		now := time.Now()

		for i, wpReq := range req.Waypoints {
			var wp models.Waypoint

			if wpReq.ID != nil {
				// Обновляем существующую точку
				// Ищем её в existingWaypoints
				found := false
				for _, existingWP := range existingWaypoints {
					if existingWP.ID == *wpReq.ID {
						wp = existingWP
						found = true
						break
					}
				}
				if !found {
					s.log.Warn("Точка не найдена, будет создана новая", "waypoint_id", wpReq.ID)
					wp.ID = uuid.New()
				}
			} else {
				// Создаём новую точку
				wp.ID = uuid.New()
			}

			wp.RouteID = routeID
			wp.CreatedAt = now
			wp.UpdatedAt = &now

			if wpReq.City != nil {
				wp.City = *wpReq.City
			}
			if wpReq.Description != nil {
				wp.Description = *wpReq.Description
			}
			if wpReq.Latitude != nil {
				wp.Latitude = *wpReq.Latitude
			}
			if wpReq.Longitude != nil {
				wp.Longitude = *wpReq.Longitude
			}
			if wpReq.DurationDays != nil {
				wp.DurationDays = *wpReq.DurationDays
			}
			if wpReq.DurationHours != nil {
				wp.DurationHours = *wpReq.DurationHours
			}
			if wpReq.OrderIndex != nil {
				wp.OrderIndex = *wpReq.OrderIndex
			} else {
				wp.OrderIndex = i
			}

			// Валидация
			if wp.Latitude < -90 || wp.Latitude > 90 || wp.Longitude < -180 || wp.Longitude > 180 {
				return nil, ErrInvalidCoordinates
			}

			waypoints = append(waypoints, wp)
		}

		// Пересчитываем общую длительность
		createWaypoints := make([]models.CreateWaypoint, len(waypoints))
		for i, wp := range waypoints {
			createWaypoints[i] = models.CreateWaypoint{
				City:          wp.City,
				Description:   wp.Description,
				Latitude:      wp.Latitude,
				Longitude:     wp.Longitude,
				DurationDays:  wp.DurationDays,
				DurationHours: wp.DurationHours,
				OrderIndex:    wp.OrderIndex,
			}
		}
		existingRoute.TotalDays, existingRoute.TotalHours = s.calculateTotalDuration(createWaypoints)
	}

	// Сохраняем изменения
	if err := s.routeRepo.Update(ctx, existingRoute, waypoints); err != nil {
		s.log.Error("Ошибка обновления маршрута", "error", err, "route_id", routeID)
		return nil, err
	}

	// Обновляем теги, если переданы
	var tags []models.Tag
	if req.Tags != nil {
		tagIDs := make([]uuid.UUID, 0, len(req.Tags))

		for _, tagName := range req.Tags {
			tag, err := s.tagRepo.GetOrCreate(ctx, tagName)
			if err != nil {
				s.log.Error("Ошибка создания/получения тега", "error", err, "tag", tagName)
				continue
			}
			tagIDs = append(tagIDs, tag.ID)
			tags = append(tags, *tag)
		}

		if err := s.tagRepo.ReplaceForRoute(ctx, routeID, tagIDs); err != nil {
			s.log.Error("Ошибка замены тегов маршрута", "error", err, "route_id", routeID)
		}
	} else {
		// Получаем существующие теги
		tags, err = s.tagRepo.GetByRouteID(ctx, routeID)
		if err != nil {
			s.log.Error("Ошибка получения тегов маршрута", "error", err, "route_id", routeID)
			tags = []models.Tag{}
		}
	}

	// Публикуем событие
	if err := s.producer.PublishRouteUpdated(ctx, routeID, userID, existingRoute.Title, existingRoute.Country, existingRoute.IsPublic); err != nil {
		s.log.Error("Ошибка публикации события route.updated", "error", err, "route_id", routeID)
	}

	s.log.Info("Маршрут успешно обновлён", "route_id", routeID)

	return &models.RouteWithDetails{
		Route:     *existingRoute,
		Waypoints: waypoints,
		Tags:      tags,
	}, nil
}

// DeleteRoute удаляет маршрут
func (s *RouteService) DeleteRoute(ctx context.Context, routeID, userID uuid.UUID) error {
	s.log.Debug("Удаление маршрута", "route_id", routeID, "user_id", userID)

	// Проверяем права
	isOwner, err := s.routeRepo.IsOwner(ctx, routeID, userID)
	if err != nil {
		s.log.Error("Ошибка проверки владельца", "error", err, "route_id", routeID)
		return err
	}
	if !isOwner {
		return ErrNotOwner
	}

	// Удаляем
	if err := s.routeRepo.Delete(ctx, routeID); err != nil {
		s.log.Error("Ошибка удаления маршрута", "error", err, "route_id", routeID)
		return err
	}

	// Публикуем событие
	if err := s.producer.PublishRouteDeleted(ctx, routeID, userID); err != nil {
		s.log.Error("Ошибка публикации события route.deleted", "error", err, "route_id", routeID)
	}

	s.log.Info("Маршрут успешно удалён", "route_id", routeID)
	return nil
}

// ToggleLike ставит или убирает лайк
func (s *RouteService) ToggleLike(ctx context.Context, routeID, userID uuid.UUID) (bool, int, error) {
	s.log.Debug("ToggleLike", "route_id", routeID, "user_id", userID)

	// Проверяем, существует ли маршрут
	exists, err := s.routeRepo.Exists(ctx, routeID)
	if err != nil {
		s.log.Error("Ошибка проверки существования маршрута", "error", err, "route_id", routeID)
		return false, 0, err
	}
	if !exists {
		return false, 0, ErrRouteNotFound
	}

	// Проверяем, лайкнул ли уже
	isLiked, err := s.likeRepo.IsLiked(ctx, userID, routeID)
	if err != nil {
		s.log.Error("Ошибка проверки лайка", "error", err, "route_id", routeID)
		return false, 0, err
	}

	if isLiked {
		// Убираем лайк
		if err := s.likeRepo.Unlike(ctx, userID, routeID); err != nil {
			s.log.Error("Ошибка снятия лайка", "error", err, "route_id", routeID)
			return false, 0, err
		}
	} else {
		// Ставим лайк
		if err := s.likeRepo.Like(ctx, userID, routeID); err != nil {
			s.log.Error("Ошибка установки лайка", "error", err, "route_id", routeID)
			return false, 0, err
		}
	}

	// Получаем новое количество лайков
	count, err := s.likeRepo.CountByRouteID(ctx, routeID)
	if err != nil {
		s.log.Error("Ошибка получения количества лайков", "error", err, "route_id", routeID)
		return !isLiked, 0, err
	}

	s.log.Info("Лайк переключён", "route_id", routeID, "user_id", userID, "now_liked", !isLiked, "count", count)
	return !isLiked, count, nil
}

// ListPublicRoutes возвращает список публичных маршрутов с фильтрацией
func (s *RouteService) ListPublicRoutes(ctx context.Context, page, size int, sort string, filters map[string]interface{}, currentUserID *uuid.UUID) ([]models.RouteWithLike, int64, error) {
	s.log.Debug("ListPublicRoutes",
		"page", page,
		"size", size,
		"sort", sort,
		"filters", filters,
		"user_id", currentUserID,
	)

	offset := (page - 1) * size
	routes, total, err := s.routeRepo.ListPublic(ctx, size, offset, sort, filters)
	if err != nil {
		s.log.Error("Ошибка получения публичных маршрутов", "error", err)
		return nil, 0, err
	}

	// Обогащаем информацией о лайках для текущего пользователя
	result := make([]models.RouteWithLike, len(routes))
	for i, route := range routes {
		s.log.Debug("Like enrichment Public Routes started")
		isLiked := false
		if currentUserID != nil {
			isLiked, err = s.likeRepo.IsLiked(ctx, *currentUserID, route.ID)
			s.log.Debug("Like enrichment Public Routes:",
				"route.ID", route.ID,
				"user_id", *currentUserID,
				"isLiked", isLiked,
			)
			if err != nil {
				s.log.Error("Ошибка проверки лайка", "error", err, "route_id", route.ID)
				isLiked = false // не фатально
			}
		}

		result[i] = models.RouteWithLike{
			Route:   route,
			IsLiked: isLiked,
		}
	}

	return result, total, nil
}

// ListUserRoutes возвращает маршруты пользователя
func (s *RouteService) ListUserRoutes(ctx context.Context, userID uuid.UUID, page, size int) ([]models.RouteWithLike, int64, error) {
	s.log.Debug("ListUserRoutes", "user_id", userID, "page", page, "size", size)

	offset := (page - 1) * size
	routes, total, err := s.routeRepo.ListByUserID(ctx, userID, size, offset)
	if err != nil {
		s.log.Error("Ошибка получения маршрутов пользователя", "error", err, "user_id", userID)
		return nil, 0, err
	}

	// Обогащаем информацией о лайках для текущего пользователя
	result := make([]models.RouteWithLike, len(routes))
	for i, route := range routes {
		s.log.Debug("Like enrichment MyRoutes started")
		isLiked := false

		isLiked, err = s.likeRepo.IsLiked(ctx, userID, route.ID)
		s.log.Debug("Like enrichment MyRoutes:",
			"route.ID", route.ID,
			"user_id", userID,
			"isLiked", isLiked,
		)
		if err != nil {
			s.log.Error("Ошибка проверки лайка", "error", err, "route_id", route.ID)
			isLiked = false // не фатально
		}

		result[i] = models.RouteWithLike{
			Route:   route,
			IsLiked: isLiked,
		}
	}

	return result, total, nil
}

// GetAllTags возвращает все теги с пагинацией
func (s *RouteService) GetAllTags(ctx context.Context, page, size int) ([]models.Tag, int64, error) {
	s.log.Debug("GetAllTags", "page", page, "size", size)

	offset := (page - 1) * size
	tags, total, err := s.tagRepo.GetAll(ctx, size, offset)
	if err != nil {
		s.log.Error("Ошибка получения тегов", "error", err)
		return nil, 0, err
	}

	return tags, total, nil
}

// GetRoutesByTag возвращает маршруты по тегу
func (s *RouteService) GetRoutesByTag(ctx context.Context, tagName string, page, size int, currentUserID *uuid.UUID) ([]models.RouteWithLike, int64, error) {
	s.log.Debug("GetRoutesByTag", "tag", tagName, "page", page, "size", size)

	offset := (page - 1) * size
	routes, total, err := s.routeRepo.ListByTag(ctx, tagName, size, offset)
	if err != nil {
		s.log.Error("Ошибка получения маршрутов по тегу", "error", err, "tag", tagName)
		return nil, 0, err
	}

	// Обогащаем информацией о лайках для текущего пользователя
	result := make([]models.RouteWithLike, len(routes))
	for i, route := range routes {
		isLiked := false
		if currentUserID != nil {
			isLiked, err = s.likeRepo.IsLiked(ctx, *currentUserID, route.ID)
			if err != nil {
				s.log.Error("Ошибка проверки лайка", "error", err, "route_id", route.ID)
				isLiked = false
			}
		}

		result[i] = models.RouteWithLike{
			Route:   route,
			IsLiked: isLiked,
		}
	}

	return result, total, nil
}

// GetPopularRoutes возвращает популярные маршруты
func (s *RouteService) GetPopularRoutes(ctx context.Context, limit int, currentUserID *uuid.UUID) ([]models.RouteWithLike, error) {
	s.log.Debug("GetPopularRoutes", "limit", limit)

	routes, err := s.routeRepo.GetPopular(ctx, limit)
	if err != nil {
		s.log.Error("Ошибка получения популярных маршрутов", "error", err)
		return nil, err
	}

	// Обогащаем информацией о лайках для текущего пользователя
	result := make([]models.RouteWithLike, len(routes))
	for i, route := range routes {
		isLiked := false
		if currentUserID != nil {
			isLiked, err = s.likeRepo.IsLiked(ctx, *currentUserID, route.ID)
			if err != nil {
				s.log.Error("Ошибка проверки лайка", "error", err, "route_id", route.ID)
				isLiked = false
			}
		}

		result[i] = models.RouteWithLike{
			Route:   route,
			IsLiked: isLiked,
		}
	}

	return result, nil
}

// GetRouteLikes возвращает список пользователей, лайкнувших маршрут
func (s *RouteService) GetRouteLikes(ctx context.Context, routeID uuid.UUID, page, size int) ([]uuid.UUID, int64, error) {
	s.log.Debug("GetRouteLikes", "route_id", routeID, "page", page, "size", size)

	offset := (page - 1) * size
	userIDs, total, err := s.likeRepo.GetUsersByRouteID(ctx, routeID, size, offset)
	if err != nil {
		s.log.Error("Ошибка получения списка лайков", "error", err, "route_id", routeID)
		return nil, 0, err
	}

	return userIDs, total, nil
}

// Вспомогательные функции

func (s *RouteService) calculateTotalDuration(waypoints []models.CreateWaypoint) (int, int) {
	totalDays := 0
	totalHours := 0
	for _, wp := range waypoints {
		totalDays += wp.DurationDays
		totalHours += wp.DurationHours
	}
	// Нормализуем часы в дни
	totalDays += totalHours / 24
	totalHours = totalHours % 24
	return totalDays, totalHours
}

func timePtr(t time.Time) *time.Time {
	return &t
}
