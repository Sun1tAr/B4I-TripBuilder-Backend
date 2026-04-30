package route

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
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

// GetRouteWithWaypointsAndTransfers получает маршрут с точками и перемещениями
func (c *Client) GetRouteWithWaypointsAndTransfers(ctx context.Context, routeID string) (*RouteWithTransfers, error) {
	url := fmt.Sprintf("%s/api/v1/routes/%s", c.baseURL, routeID)

	c.log.Debug("RouteClient.GetRouteWithWaypointsAndTransfers: запрос",
		"url", url,
		"route_id", routeID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
		req.Header.Set("X-User-ID", userID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("route service returned %d", resp.StatusCode)
	}

	var route RouteWithTransfers
	if err := json.NewDecoder(resp.Body).Decode(&route); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.log.Debug("RouteClient.GetRouteWithWaypointsAndTransfers: маршрут получен",
		"route_id", routeID,
		"waypoints", len(route.Waypoints),
		"transfers", len(route.Transfers),
	)

	return &route, nil
}

// ========== СТРУКТУРЫ ДЛЯ ОТВЕТА ОТ ROUTE SERVICE ==========

// RouteWithTransfers полный ответ от Route Service с перемещениями
type RouteWithTransfers struct {
	ID          uuid.UUID       `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Country     string          `json:"country,omitempty"`
	ImageURL    string          `json:"image_url,omitempty"`
	UserID      uuid.UUID       `json:"user_id"`
	IsPublic    bool            `json:"is_public"`
	Waypoints   []RouteWaypoint `json:"waypoints"`
	Transfers   []Transfer      `json:"transfers,omitempty"`
}

// RouteWaypoint точка маршрута от Route Service
type RouteWaypoint struct {
	ID            uuid.UUID `json:"id"`
	City          string    `json:"city"`
	Description   string    `json:"description,omitempty"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	DurationDays  int       `json:"duration_days"`
	DurationHours int       `json:"duration_hours"`
	OrderIndex    int       `json:"order_index"`
}

// Transfer перемещение от Route Service
type Transfer struct {
	ID              uuid.UUID `json:"id"`
	FromWaypointID  uuid.UUID `json:"from_waypoint_id"`
	ToWaypointID    uuid.UUID `json:"to_waypoint_id"`
	DurationHours   int       `json:"duration_hours"`
	DurationMinutes int       `json:"duration_minutes"`
	TransportType   string    `json:"transport_type"`
	OrderIndex      int       `json:"order_index"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}
