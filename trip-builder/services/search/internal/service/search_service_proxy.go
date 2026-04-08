package service

import (
	"context"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/client"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type ProxySearchService struct {
	routeClient *client.RouteClient
	eventClient *client.EventClient
	userClient  *client.UserClient
	cfg         *config.Config
	log         *logger.Logger
}

func NewProxySearchService(cfg *config.Config, log *logger.Logger) *ProxySearchService {
	return &ProxySearchService{
		routeClient: client.NewRouteClient(cfg.RouteServiceURL, cfg.HTTPClientTimeout, log),
		eventClient: client.NewEventClient(cfg.EventServiceURL, cfg.HTTPClientTimeout, log),
		userClient:  client.NewUserClient(cfg.UserServiceURL, cfg.HTTPClientTimeout, log),
		cfg:         cfg,
		log:         log,
	}
}

// SearchRoutes проксирует запрос в Route Service
func (s *ProxySearchService) SearchRoutes(ctx context.Context, req *models.RouteSearchRequest) (*models.RouteSearchResponse, error) {
	return s.routeClient.SearchRoutes(ctx, req)
}

// SearchEvents проксирует запрос в Event Service
func (s *ProxySearchService) SearchEvents(ctx context.Context, req *models.EventSearchRequest) (*models.EventSearchResponse, error) {
	return s.eventClient.SearchEvents(ctx, req)
}

// SearchUsers проксирует запрос в User Service
func (s *ProxySearchService) SearchUsers(ctx context.Context, req *models.UserSearchRequest) (*models.UserSearchResponse, error) {
	return s.userClient.SearchUsers(ctx, req)
}

// Suggest – в прокси-режиме не поддерживается
func (s *ProxySearchService) Suggest(ctx context.Context, req *models.SuggestRequest) (*models.SuggestResponse, error) {
	return &models.SuggestResponse{Suggestions: []models.Suggestion{}}, nil
}

// Методы для совместимости с интерфейсом (в прокси-режиме не используются)
func (s *ProxySearchService) IndexRoute(ctx context.Context, doc *models.RouteDocument) error {
	return nil
}

func (s *ProxySearchService) DeleteRoute(ctx context.Context, id string) error {
	return nil
}

func (s *ProxySearchService) IndexEvent(ctx context.Context, doc *models.EventDocument) error {
	return nil
}

func (s *ProxySearchService) DeleteEvent(ctx context.Context, id string) error {
	return nil
}

func (s *ProxySearchService) IndexUser(ctx context.Context, doc *models.UserDocument) error {
	return nil
}

func (s *ProxySearchService) DeleteUser(ctx context.Context, id string) error {
	return nil
}

func (s *ProxySearchService) FullReindexES(ctx context.Context) error {
	return nil
}

func (s *ProxySearchService) FullReindexRedis(ctx context.Context) error {
	return nil
}
