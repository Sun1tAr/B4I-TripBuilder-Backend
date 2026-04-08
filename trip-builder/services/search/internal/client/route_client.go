package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type RouteClient struct {
	baseURL    string
	httpClient *http.Client
	log        *logger.Logger
}

func NewRouteClient(baseURL string, timeout time.Duration, log *logger.Logger) *RouteClient {
	return &RouteClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
		log:        log,
	}
}

// SearchRoutes выполняет поиск маршрутов через Route Service
func (c *RouteClient) SearchRoutes(ctx context.Context, req *models.RouteSearchRequest) (*models.RouteSearchResponse, error) {
	params := url.Values{}

	if req.Query != "" {
		params.Set("q", req.Query)
	}
	if req.Country != "" {
		params.Set("country", req.Country)
	}
	if len(req.Tags) > 0 {
		params.Set("tags", strings.Join(req.Tags, ","))
	}
	if req.DurationFrom != nil {
		params.Set("duration_from", strconv.Itoa(*req.DurationFrom))
	}
	if req.DurationTo != nil {
		params.Set("duration_to", strconv.Itoa(*req.DurationTo))
	}
	if req.Sort != "" {
		params.Set("sort", req.Sort)
	}
	params.Set("page", strconv.Itoa(req.Page))
	params.Set("size", strconv.Itoa(req.Size))

	fullURL := fmt.Sprintf("%s/api/v1/routes?%s", c.baseURL, params.Encode())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	// Пробрасываем user_id из контекста (устанавливается middleware)
	if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
		httpReq.Header.Set("X-User-ID", userID)
	}

	c.log.Debug("RouteClient.SearchRoutes", "url", fullURL)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("route service returned %d: %s", resp.StatusCode, string(body))
	}

	var result models.RouteSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetRouteByID получает маршрут по ID
func (c *RouteClient) GetRouteByID(ctx context.Context, id string) (*models.RouteDocument, error) {
	fullURL := fmt.Sprintf("%s/api/v1/routes/%s", c.baseURL, id)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
		httpReq.Header.Set("X-User-ID", userID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("route service returned %d", resp.StatusCode)
	}

	var result models.RouteDocument
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
