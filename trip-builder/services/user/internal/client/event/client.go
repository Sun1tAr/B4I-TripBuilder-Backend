package event

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/retry"
)

type Event struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Status    string    `json:"status"`
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

// GetUserEvents пытается получить события синхронно, при недоступности Event Service отправляет в retry queue
func (c *Client) GetUserEvents(ctx context.Context, userID string) ([]Event, error) {
	url := fmt.Sprintf("%s/api/v1/events/user/%s", c.baseURL, userID)

	// Пробуем синхронно
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var events []Event
		if err := json.NewDecoder(resp.Body).Decode(&events); err == nil {
			return events, nil
		}
	}

	// Если не получилось, логируем и отправляем в retry
	c.log.Warn("Синхронный вызов Event Service не удался, отправляем в retry", "user_id", userID, "error", err)

	// Сохраняем запрос в retry очередь
	failedReq := retry.FailedRequest{
		Method:      "GET",
		URL:         url,
		Headers:     map[string]string{},
		Body:        nil,
		Attempts:    0,
		LastError:   err.Error(),
		LastAttempt: time.Now(),
	}

	if err := c.asyncHTTP.Enqueue(ctx, failedReq); err != nil {
		c.log.Error("Не удалось отправить запрос в retry queue", "error", err)
	}

	// Возвращаем пустой список как fallback
	return []Event{}, nil
}