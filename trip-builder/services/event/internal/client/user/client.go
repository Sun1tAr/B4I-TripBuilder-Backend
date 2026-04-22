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

// ProfileResponse ответ от User Service
type ProfileResponse struct {
	UserID    uuid.UUID `json:"user_id"`
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Bio       string    `json:"bio,omitempty"`
}

// Client клиент для User Service
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

// GetProfile получает профиль пользователя по ID
func (c *Client) GetProfile(ctx context.Context, userID uuid.UUID) (*ProfileResponse, error) {
	url := fmt.Sprintf("%s/api/v1/users/%s", c.baseURL, userID)

	c.log.Debug("UserClient.GetProfile: запрос",
		"url", url,
		"user_id", userID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-User-ID", userID.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("UserClient.GetProfile: ошибка запроса", "error", err)
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		c.log.Debug("UserClient.GetProfile: пользователь не найден", "user_id", userID)
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("UserClient.GetProfile: неожиданный статус",
			"status", resp.StatusCode,
			"user_id", userID,
		)
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

// GetFullName возвращает полное имя пользователя
func (c *Client) GetFullName(ctx context.Context, userID uuid.UUID) string {
	profile, err := c.GetProfile(ctx, userID)
	if err != nil || profile == nil {
		return "Участник"
	}

	name := fmt.Sprintf("%s %s", profile.FirstName, profile.LastName)
	if name == " " {
		return "Участник"
	}
	return name
}
