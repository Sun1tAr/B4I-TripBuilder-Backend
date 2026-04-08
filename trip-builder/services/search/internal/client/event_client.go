package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type EventClient struct {
	baseURL    string
	httpClient *http.Client
	log        *logger.Logger
}

func NewEventClient(baseURL string, timeout time.Duration, log *logger.Logger) *EventClient {
	return &EventClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
		log:        log,
	}
}

// SearchEvents выполняет поиск публичных поездок
func (c *EventClient) SearchEvents(ctx context.Context, req *models.EventSearchRequest) (*models.EventSearchResponse, error) {
	params := url.Values{}

	if req.Query != "" {
		params.Set("q", req.Query)
	}
	if req.Status != "" {
		params.Set("status", req.Status)
	}
	params.Set("page", strconv.Itoa(req.Page))
	params.Set("size", strconv.Itoa(req.Size))

	fullURL := fmt.Sprintf("%s/api/v1/events/public?%s", c.baseURL, params.Encode())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("event service returned %d: %s", resp.StatusCode, string(body))
	}

	var result models.EventSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
