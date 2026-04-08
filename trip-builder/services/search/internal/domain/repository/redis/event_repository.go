package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type EventRepository struct {
	redis *Client
	log   *logger.Logger
}

func NewEventRepository(redis *Client, log *logger.Logger) *EventRepository {
	return &EventRepository{
		redis: redis,
		log:   log,
	}
}

func (r *EventRepository) EnsureIndex(ctx context.Context) error {
	return nil
}

func (r *EventRepository) Index(ctx context.Context, doc *models.EventDocument) error {
	if !doc.IsPublic {
		return nil
	}

	data := map[string]interface{}{
		"id":         doc.ID,
		"title":      doc.Title,
		"route_id":   doc.RouteID,
		"start_date": doc.StartDate.Format(time.RFC3339),
		"status":     doc.Status,
		"is_public":  doc.IsPublic,
	}
	return r.redis.HSet(ctx, fmt.Sprintf("event:%s", doc.ID), data)
}

func (r *EventRepository) Delete(ctx context.Context, id string) error {
	return r.redis.Del(ctx, fmt.Sprintf("event:%s", id))
}

func (r *EventRepository) Search(ctx context.Context, req *models.EventSearchRequest) (*models.EventSearchResponse, error) {
	// Для событий в Redis – простая фильтрация по ключам
	keys, err := r.redis.Keys(ctx, "event:*")
	if err != nil {
		return nil, err
	}

	items := []models.EventHit{}
	for _, key := range keys {
		data, err := r.redis.HGetAll(ctx, key)
		if err != nil {
			continue
		}
		if len(data) == 0 {
			continue
		}

		isPublic := data["is_public"] == "true"
		if !isPublic {
			continue
		}

		status := data["status"]
		if status != "completed" {
			continue
		}

		title := data["title"]
		if req.Query != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(req.Query)) {
			continue
		}

		startDate, _ := time.Parse(time.RFC3339, data["start_date"])

		items = append(items, models.EventHit{
			ID:        data["id"],
			Title:     title,
			RouteID:   data["route_id"],
			StartDate: startDate,
			Status:    status,
		})
	}

	// Сортировка по дате
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].StartDate.Before(items[j].StartDate) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	total := int64(len(items))
	start := (req.Page - 1) * req.Size
	end := start + req.Size
	if start > len(items) {
		start = len(items)
	}
	if end > len(items) {
		end = len(items)
	}
	paged := items[start:end]

	totalPages := int(total) / req.Size
	if int(total)%req.Size > 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	return &models.EventSearchResponse{
		Items:      paged,
		Total:      total,
		Page:       req.Page,
		Size:       req.Size,
		TotalPages: totalPages,
	}, nil
}

func (r *EventRepository) ReindexAll(ctx context.Context, events []*models.EventDocument) error {
	r.log.Info("Переиндексация событий в Redis", "count", len(events))

	// Очищаем старые события
	keys, err := r.redis.Keys(ctx, "event:*")
	if err == nil {
		for _, key := range keys {
			_ = r.redis.Del(ctx, key)
		}
	}

	for _, event := range events {
		if err := r.Index(ctx, event); err != nil {
			r.log.Error("Ошибка индексации события в Redis", "error", err, "event_id", event.ID)
		}
	}
	return nil
}
