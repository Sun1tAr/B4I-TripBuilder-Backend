package event

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/retry"
	"github.com/google/uuid"
)

type Event struct {
	ID                uuid.UUID  `json:"id"`
	Title             string     `json:"title"`
	Description       string     `json:"description,omitempty"`
	CoverURL          string     `json:"cover_url,omitempty"`
	OwnerID           uuid.UUID  `json:"owner_id"`
	StartDate         time.Time  `json:"start_date"`
	EndDate           time.Time  `json:"end_date"`
	IsGroup           bool       `json:"is_group"`
	Status            string     `json:"status"`
	IsPublic          bool       `json:"is_public"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty"`
	TotalParticipants int        `json:"total_participants"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	asyncHTTP  *retry.AsyncHTTPClient
	log        *logger.Logger
}

func NewClient(baseURL string, retryCfg retry.RetryConfig, log *logger.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		asyncHTTP: retry.NewAsyncHTTPClient(retryCfg, log),
		log:       log,
	}
}

// GetUserEvents получает события пользователя из Event Service
func (c *Client) GetUserEvents(ctx context.Context, userID string) ([]Event, error) {
	url := fmt.Sprintf("%s/api/v1/events/my", c.baseURL)
	c.log.Debug("EventClient.GetUserEvents: запрос", "url", url, "user_id", userID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Добавляем заголовок X-User-ID
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}

	// Пробуем синхронно
	resp, err := c.httpClient.Do(req)
	c.log.Debug("EventClient.GetUserEvents: вернулся ответ", "response", resp.Body)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()

		var result struct {
			Items []Event `json:"items"`
			Total int64   `json:"total"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			c.log.Debug("EventClient.GetUserEvents: успешно", "user_id", userID, "count", len(result.Items))
			return result.Items, nil
		}
	}

	c.log.Error("EventClient.GetUserEvents: неуспешно")

	return []Event{}, nil
}
