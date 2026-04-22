package user

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type ProfileResponse struct {
	UserID    uuid.UUID `json:"user_id"`
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	log        *logger.Logger
}

func NewClient(baseURL string, timeout time.Duration, log *logger.Logger) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
		log:        log,
	}
}

// GetProfile получает профиль пользователя по ID
func (c *Client) GetProfile(ctx context.Context, userID uuid.UUID) (*ProfileResponse, error) {
	url := fmt.Sprintf("%s/api/v1/users/%s", c.baseURL, userID)

	c.log.Debug("UserClient.GetProfile: запрос", "url", url, "user_id", userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.log.Error("UserClient.GetProfile: ошибка создания запроса", "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-User-ID", userID.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("UserClient.GetProfile: ошибка выполнения запроса", "error", err)
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		c.log.Debug("UserClient.GetProfile: профиль не найден", "user_id", userID)
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("UserClient.GetProfile: неожиданный статус", "status", resp.StatusCode)
		return nil, fmt.Errorf("user service returned %d", resp.StatusCode)
	}

	var profile ProfileResponse
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		c.log.Error("UserClient.GetProfile: ошибка декодирования", "error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.log.Debug("UserClient.GetProfile: профиль получен",
		"user_id", userID,
		"first_name", profile.FirstName,
		"last_name", profile.LastName,
	)

	return &profile, nil
}
