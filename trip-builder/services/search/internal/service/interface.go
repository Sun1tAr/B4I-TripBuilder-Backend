package service

import (
	"context"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
)

type SearchServiceInterface interface {
	// Маршруты
	IndexRoute(ctx context.Context, doc *models.RouteDocument) error
	DeleteRoute(ctx context.Context, id string) error
	SearchRoutes(ctx context.Context, req *models.RouteSearchRequest) (*models.RouteSearchResponse, error)

	// События
	IndexEvent(ctx context.Context, doc *models.EventDocument) error
	DeleteEvent(ctx context.Context, id string) error
	SearchEvents(ctx context.Context, req *models.EventSearchRequest) (*models.EventSearchResponse, error)

	// Пользователи
	IndexUser(ctx context.Context, doc *models.UserDocument) error
	DeleteUser(ctx context.Context, id string) error
	SearchUsers(ctx context.Context, req *models.UserSearchRequest) (*models.UserSearchResponse, error)

	// Suggest
	Suggest(ctx context.Context, req *models.SuggestRequest) (*models.SuggestResponse, error)

	// Реиндексация
	FullReindexES(ctx context.Context) error
	FullReindexRedis(ctx context.Context) error
}
