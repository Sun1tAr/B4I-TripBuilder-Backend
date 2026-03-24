package route

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	log        *logger.Logger
}

func NewClient(baseURL string, timeout time.Duration, log *logger.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		log: log,
	}
}

// GetRouteWithWaypoints получает маршрут с точками от Route Service
func (c *Client) GetRouteWithWaypoints(ctx context.Context, routeID string) (*models.RouteWithWaypoints, error) {
	url := fmt.Sprintf("%s/api/v1/routes/%s", c.baseURL, routeID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.log.Error("RouteClient.GetRouteWithWaypoints: ошибка создания запроса", "error", err, "route_id", routeID)
		return nil, err
	}

	// Передаём X-User-ID из контекста, если он есть
	if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
		req.Header.Set("X-User-ID", userID)
		c.log.Debug("RouteClient.GetRouteWithWaypoints: передаём X-User-ID", "user_id", userID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("RouteClient.GetRouteWithWaypoints: ошибка запроса", "error", err, "route_id", routeID)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.log.Error("RouteClient.GetRouteWithWaypoints: неожиданный статус", "status", resp.StatusCode, "route_id", routeID)
		return nil, fmt.Errorf("route service returned %d", resp.StatusCode)
	}

	var route models.RouteWithWaypoints
	if err := json.NewDecoder(resp.Body).Decode(&route); err != nil {
		c.log.Error("RouteClient.GetRouteWithWaypoints: ошибка декодирования", "error", err, "route_id", routeID)
		return nil, err
	}

	c.log.Debug("RouteClient.GetRouteWithWaypoints: маршрут получен", "route_id", routeID, "waypoints_count", len(route.Waypoints))
	return &route, nil
}
