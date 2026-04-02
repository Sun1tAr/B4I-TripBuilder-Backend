package event

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

// IsEventOwner проверяет, является ли пользователь владельцем события
func (c *Client) IsEventOwner(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/events/%s/owner?user_id=%s", c.baseURL, eventID, userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	// Добавляем заголовок X-User-ID
	req.Header.Set("X-User-ID", userID.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("IsEventOwner: ошибка запроса", "error", err)
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if resp.StatusCode == http.StatusUnauthorized {
		c.log.Error("IsEventOwner: неавторизован", "user_id", userID)
		return false, fmt.Errorf("unauthorized")
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("event service returned %d", resp.StatusCode)
	}

	var result struct {
		IsOwner bool `json:"is_owner"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return result.IsOwner, nil
}

// IsEventParticipant проверяет, является ли пользователь активным участником события
func (c *Client) IsEventParticipant(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/events/%s/participant?user_id=%s", c.baseURL, eventID, userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	// Добавляем заголовок X-User-ID
	req.Header.Set("X-User-ID", userID.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("IsEventParticipant: ошибка запроса", "error", err)
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if resp.StatusCode == http.StatusUnauthorized {
		c.log.Error("IsEventParticipant: неавторизован", "user_id", userID)
		return false, fmt.Errorf("unauthorized")
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("event service returned %d", resp.StatusCode)
	}

	var result struct {
		IsParticipant bool `json:"is_participant"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return result.IsParticipant, nil
}
