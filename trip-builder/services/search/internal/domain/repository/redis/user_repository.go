package redis

import (
	"context"
	"fmt"
	"strings"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type UserRepository struct {
	redis *Client
	log   *logger.Logger
}

func NewUserRepository(redis *Client, log *logger.Logger) *UserRepository {
	return &UserRepository{
		redis: redis,
		log:   log,
	}
}

func (r *UserRepository) EnsureIndex(ctx context.Context) error {
	return nil
}

func (r *UserRepository) Index(ctx context.Context, doc *models.UserDocument) error {
	data := map[string]interface{}{
		"id":         doc.ID,
		"first_name": doc.FirstName,
		"last_name":  doc.LastName,
		"bio":        doc.Bio,
	}
	return r.redis.HSet(ctx, fmt.Sprintf("user:%s", doc.ID), data)
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	return r.redis.Del(ctx, fmt.Sprintf("user:%s", id))
}

func (r *UserRepository) Search(ctx context.Context, req *models.UserSearchRequest) (*models.UserSearchResponse, error) {
	keys, err := r.redis.Keys(ctx, "user:*")
	if err != nil {
		return nil, err
	}

	items := []models.UserHit{}
	for _, key := range keys {
		data, err := r.redis.HGetAll(ctx, key)
		if err != nil {
			continue
		}
		if len(data) == 0 {
			continue
		}

		firstName := data["first_name"]
		lastName := data["last_name"]
		fullName := firstName + " " + lastName

		if req.Query != "" {
			queryLower := strings.ToLower(req.Query)
			if !strings.Contains(strings.ToLower(fullName), queryLower) &&
				!strings.Contains(strings.ToLower(data["bio"]), queryLower) {
				continue
			}
		}

		items = append(items, models.UserHit{
			ID:        data["id"],
			FirstName: firstName,
			LastName:  lastName,
			Bio:       data["bio"],
		})
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

	return &models.UserSearchResponse{
		Items:      paged,
		Total:      total,
		Page:       req.Page,
		Size:       req.Size,
		TotalPages: totalPages,
	}, nil
}

func (r *UserRepository) ReindexAll(ctx context.Context, users []*models.UserDocument) error {
	r.log.Info("Переиндексация пользователей в Redis", "count", len(users))

	keys, err := r.redis.Keys(ctx, "user:*")
	if err == nil {
		for _, key := range keys {
			_ = r.redis.Del(ctx, key)
		}
	}

	for _, user := range users {
		if err := r.Index(ctx, user); err != nil {
			r.log.Error("Ошибка индексации пользователя в Redis", "error", err, "user_id", user.ID)
		}
	}
	return nil
}
