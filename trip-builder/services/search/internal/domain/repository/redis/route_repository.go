package redis

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type RouteRepository struct {
	redis *Client
	log   *logger.Logger
}

func NewRouteRepository(redis *Client, log *logger.Logger) *RouteRepository {
	return &RouteRepository{
		redis: redis,
		log:   log,
	}
}

func (r *RouteRepository) EnsureIndex(ctx context.Context) error {
	// Redis не требует создания индексов
	return nil
}

func (r *RouteRepository) Index(ctx context.Context, doc *models.RouteDocument) error {
	// Сохраняем документ
	if err := r.redis.SaveRoute(ctx, doc); err != nil {
		return err
	}

	// Индексируем по словам из title и description
	words := extractWords(doc.Title + " " + doc.Description)
	for _, word := range words {
		key := fmt.Sprintf("idx:route:word:%s", word)
		if err := r.redis.SAdd(ctx, key, doc.ID); err != nil {
			r.log.Error("Ошибка индексации слова", "word", word, "error", err)
		}
	}

	// Индексируем по тегам
	for _, tag := range doc.Tags {
		key := fmt.Sprintf("idx:route:tag:%s", tag)
		if err := r.redis.SAdd(ctx, key, doc.ID); err != nil {
			r.log.Error("Ошибка индексации тега", "tag", tag, "error", err)
		}
	}

	// Индексируем по стране
	if doc.Country != "" {
		key := fmt.Sprintf("idx:route:country:%s", doc.Country)
		if err := r.redis.SAdd(ctx, key, doc.ID); err != nil {
			r.log.Error("Ошибка индексации страны", "country", doc.Country, "error", err)
		}
	}

	// Индексируем по пользователю (для приватных маршрутов)
	if !doc.IsPublic {
		key := fmt.Sprintf("idx:route:user:%s", doc.UserID)
		if err := r.redis.SAdd(ctx, key, doc.ID); err != nil {
			r.log.Error("Ошибка индексации пользователя", "user_id", doc.UserID, "error", err)
		}
	}

	return nil
}

func (r *RouteRepository) Delete(ctx context.Context, id string) error {
	// Получаем документ для удаления из индексов
	doc, err := r.redis.GetRoute(ctx, id)
	if err != nil {
		r.log.Error("Ошибка получения маршрута для удаления", "error", err)
	}

	// Удаляем из индексов
	if doc != nil {
		words := extractWords(doc.Title + " " + doc.Description)
		for _, word := range words {
			key := fmt.Sprintf("idx:route:word:%s", word)
			_ = r.redis.SRem(ctx, key, id)
		}
		for _, tag := range doc.Tags {
			key := fmt.Sprintf("idx:route:tag:%s", tag)
			_ = r.redis.SRem(ctx, key, id)
		}
		if doc.Country != "" {
			key := fmt.Sprintf("idx:route:country:%s", doc.Country)
			_ = r.redis.SRem(ctx, key, id)
		}
		if !doc.IsPublic {
			key := fmt.Sprintf("idx:route:user:%s", doc.UserID)
			_ = r.redis.SRem(ctx, key, id)
		}
	}

	// Удаляем документ
	return r.redis.DeleteRoute(ctx, id)
}

func (r *RouteRepository) Search(ctx context.Context, req *models.RouteSearchRequest) (*models.RouteSearchResponse, error) {
	var candidateSets []string

	// Поиск по словам
	if req.Query != "" {
		words := extractWords(req.Query)
		for _, word := range words {
			candidateSets = append(candidateSets, fmt.Sprintf("idx:route:word:%s", word))
		}
	} else {
		// Если нет поискового запроса, берём все маршруты
		// В Redis сложно получить все ID, поэтому используем временный подход
		return r.searchByFilters(ctx, req)
	}

	if len(candidateSets) == 0 {
		return &models.RouteSearchResponse{Items: []models.RouteHit{}, Total: 0, Page: req.Page, Size: req.Size, TotalPages: 1}, nil
	}

	// Пересечение множеств
	ids, err := r.redis.SInter(ctx, candidateSets...)
	if err != nil {
		return nil, err
	}

	// Применяем фильтры
	ids, err = r.applyFilters(ctx, ids, req)
	if err != nil {
		return nil, err
	}

	// Применяем права доступа
	ids, err = r.applyAccessControl(ctx, ids, req.UserID)
	if err != nil {
		return nil, err
	}

	// Сортировка
	ids = r.sortRoutes(ctx, ids, req)

	// Пагинация
	total := int64(len(ids))
	start := (req.Page - 1) * req.Size
	end := start + req.Size
	if start > len(ids) {
		start = len(ids)
	}
	if end > len(ids) {
		end = len(ids)
	}
	pagedIDs := ids[start:end]

	// Получаем документы
	items := make([]models.RouteHit, 0, len(pagedIDs))
	for _, id := range pagedIDs {
		doc, err := r.redis.GetRoute(ctx, id)
		if err != nil || doc == nil {
			continue
		}
		items = append(items, models.RouteHit{
			ID:          doc.ID,
			Title:       doc.Title,
			Description: doc.Description,
			Country:     doc.Country,
			Tags:        doc.Tags,
			TotalDays:   doc.TotalDays,
			UserID:      doc.UserID,
			LikesCount:  doc.LikesCount,
			CreatedAt:   doc.CreatedAt,
		})
	}

	totalPages := int(total) / req.Size
	if int(total)%req.Size > 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	return &models.RouteSearchResponse{
		Items:      items,
		Total:      total,
		Page:       req.Page,
		Size:       req.Size,
		TotalPages: totalPages,
	}, nil
}

func (r *RouteRepository) applyFilters(ctx context.Context, ids []string, req *models.RouteSearchRequest) ([]string, error) {
	result := ids

	// Фильтр по стране
	if req.Country != "" {
		countrySet := fmt.Sprintf("idx:route:country:%s", req.Country)
		countryIDs, err := r.redis.SMembers(ctx, countrySet)
		if err != nil {
			return nil, err
		}
		result = intersect(result, countryIDs)
	}

	// Фильтр по тегам
	for _, tag := range req.Tags {
		tagSet := fmt.Sprintf("idx:route:tag:%s", tag)
		tagIDs, err := r.redis.SMembers(ctx, tagSet)
		if err != nil {
			return nil, err
		}
		result = intersect(result, tagIDs)
	}

	// Фильтр по длительности (требует загрузки документов)
	if req.DurationFrom != nil || req.DurationTo != nil {
		filtered := []string{}
		for _, id := range result {
			doc, err := r.redis.GetRoute(ctx, id)
			if err != nil || doc == nil {
				continue
			}
			if req.DurationFrom != nil && doc.TotalDays < *req.DurationFrom {
				continue
			}
			if req.DurationTo != nil && doc.TotalDays > *req.DurationTo {
				continue
			}
			filtered = append(filtered, id)
		}
		result = filtered
	}

	return result, nil
}

func (r *RouteRepository) applyAccessControl(ctx context.Context, ids []string, userID string) ([]string, error) {
	if userID == "" {
		// Анонимный пользователь – только публичные
		publicIDs := []string{}
		for _, id := range ids {
			doc, err := r.redis.GetRoute(ctx, id)
			if err != nil || doc == nil {
				continue
			}
			if doc.IsPublic {
				publicIDs = append(publicIDs, id)
			}
		}
		return publicIDs, nil
	}

	// Авторизованный пользователь – публичные + свои приватные
	userSet := fmt.Sprintf("idx:route:user:%s", userID)
	userIDs, err := r.redis.SMembers(ctx, userSet)
	if err != nil {
		userIDs = []string{}
	}

	userMap := make(map[string]bool)
	for _, id := range userIDs {
		userMap[id] = true
	}

	result := []string{}
	for _, id := range ids {
		doc, err := r.redis.GetRoute(ctx, id)
		if err != nil || doc == nil {
			continue
		}
		if doc.IsPublic || userMap[id] {
			result = append(result, id)
		}
	}
	return result, nil
}

func (r *RouteRepository) sortRoutes(ctx context.Context, ids []string, req *models.RouteSearchRequest) []string {
	if req.Sort == "date" {
		// Сортировка по дате (требует загрузки документов)
		items := []struct {
			id        string
			createdAt time.Time
		}{}
		for _, id := range ids {
			doc, err := r.redis.GetRoute(ctx, id)
			if err != nil || doc == nil {
				continue
			}
			items = append(items, struct {
				id        string
				createdAt time.Time
			}{id, doc.CreatedAt})
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].createdAt.After(items[j].createdAt)
		})
		result := make([]string, len(items))
		for i, item := range items {
			result[i] = item.id
		}
		return result
	}
	// По умолчанию без сортировки (как есть)
	return ids
}

func (r *RouteRepository) searchByFilters(ctx context.Context, req *models.RouteSearchRequest) (*models.RouteSearchResponse, error) {
	// Получаем все ID маршрутов (можно оптимизировать с помощью отдельного сета всех ID)
	// Для MVP – грузим все и фильтруем
	keys, err := r.redis.Keys(ctx, "route:*")
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(keys))
	for i, key := range keys {
		ids[i] = strings.TrimPrefix(key, "route:")
	}

	ids, err = r.applyFilters(ctx, ids, req)
	if err != nil {
		return nil, err
	}
	ids, err = r.applyAccessControl(ctx, ids, req.UserID)
	if err != nil {
		return nil, err
	}
	ids = r.sortRoutes(ctx, ids, req)

	total := int64(len(ids))
	start := (req.Page - 1) * req.Size
	end := start + req.Size
	if start > len(ids) {
		start = len(ids)
	}
	if end > len(ids) {
		end = len(ids)
	}
	pagedIDs := ids[start:end]

	items := make([]models.RouteHit, 0, len(pagedIDs))
	for _, id := range pagedIDs {
		doc, err := r.redis.GetRoute(ctx, id)
		if err != nil || doc == nil {
			continue
		}
		items = append(items, models.RouteHit{
			ID:          doc.ID,
			Title:       doc.Title,
			Description: doc.Description,
			Country:     doc.Country,
			Tags:        doc.Tags,
			TotalDays:   doc.TotalDays,
			UserID:      doc.UserID,
			LikesCount:  doc.LikesCount,
			CreatedAt:   doc.CreatedAt,
		})
	}

	totalPages := int(total) / req.Size
	if int(total)%req.Size > 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	return &models.RouteSearchResponse{
		Items:      items,
		Total:      total,
		Page:       req.Page,
		Size:       req.Size,
		TotalPages: totalPages,
	}, nil
}

func (r *RouteRepository) Suggest(ctx context.Context, query string, size int) ([]models.Suggestion, error) {
	// Suggest не поддерживается в Redis
	return []models.Suggestion{}, nil
}

func (r *RouteRepository) ReindexAll(ctx context.Context, routes []*models.RouteDocument) error {
	r.log.Info("Переиндексация маршрутов в Redis", "count", len(routes))

	// Очищаем старые индексы
	patterns := []string{"idx:route:word:*", "idx:route:tag:*", "idx:route:country:*", "idx:route:user:*"}
	for _, pattern := range patterns {
		keys, err := r.redis.Keys(ctx, pattern)
		if err != nil {
			continue
		}
		for _, key := range keys {
			_ = r.redis.Del(ctx, key)
		}
	}

	// Индексируем заново
	for _, route := range routes {
		if err := r.Index(ctx, route); err != nil {
			r.log.Error("Ошибка индексации маршрута в Redis", "error", err, "route_id", route.ID)
		}
	}
	return nil
}

// Вспомогательные функции
func extractWords(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	unique := make(map[string]bool)
	result := []string{}
	for _, w := range words {
		w = strings.Trim(w, ".,!?;:()\"'")
		if len(w) > 2 && !unique[w] {
			unique[w] = true
			result = append(result, w)
		}
	}
	return result
}

func intersect(a, b []string) []string {
	set := make(map[string]bool)
	for _, item := range b {
		set[item] = true
	}
	result := []string{}
	for _, item := range a {
		if set[item] {
			result = append(result, item)
		}
	}
	return result
}
