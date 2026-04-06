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
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
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

	// Если не получилось, логируем и отправляем в retry
	c.log.Warn("Синхронный вызов Event Service не удался, отправляем в retry",
		"user_id", userID,
		"error", err,
	)

	// Сохраняем запрос в retry очередь
	failedReq := retry.FailedRequest{
		Method:      "GET",
		URL:         url,
		Headers:     map[string]string{"X-User-ID": userID},
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
