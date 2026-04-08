package repository

import (
	"context"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
)

type RouteRepository interface {
	EnsureIndex(ctx context.Context) error
	Index(ctx context.Context, doc *models.RouteDocument) error
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, req *models.RouteSearchRequest) (*models.RouteSearchResponse, error)
	Suggest(ctx context.Context, query string, size int) ([]models.Suggestion, error)
	ReindexAll(ctx context.Context, routes []*models.RouteDocument) error
}

type EventRepository interface {
	EnsureIndex(ctx context.Context) error
	Index(ctx context.Context, doc *models.EventDocument) error
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, req *models.EventSearchRequest) (*models.EventSearchResponse, error)
	ReindexAll(ctx context.Context, events []*models.EventDocument) error
}

type UserRepository interface {
	EnsureIndex(ctx context.Context) error
	Index(ctx context.Context, doc *models.UserDocument) error
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, req *models.UserSearchRequest) (*models.UserSearchResponse, error)
	ReindexAll(ctx context.Context, users []*models.UserDocument) error
}
