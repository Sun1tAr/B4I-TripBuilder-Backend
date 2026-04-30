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

// UserProfile информация о пользователе
type UserProfile struct {
	UserID    uuid.UUID `json:"user_id"`
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
	Email     string    `json:"email,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
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

// GetUserByID получает профиль пользователя по ID
func (c *Client) GetUserByID(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	url := fmt.Sprintf("%s/api/v1/users/%s", c.baseURL, userID)

	c.log.Debug("UserClient.GetUserByID: запрос",
		"url", url,
		"user_id", userID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Пробрасываем X-User-ID для внутренней авторизации
	req.Header.Set("X-User-ID", userID.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("UserClient.GetUserByID: ошибка запроса", "error", err)
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		c.log.Debug("UserClient.GetUserByID: пользователь не найден", "user_id", userID)
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("UserClient.GetUserByID: неожиданный статус",
			"status", resp.StatusCode,
			"user_id", userID,
		)
		return nil, fmt.Errorf("user service returned %d", resp.StatusCode)
	}

	var profile UserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		c.log.Error("UserClient.GetUserByID: ошибка декодирования", "error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.log.Debug("UserClient.GetUserByID: профиль получен",
		"user_id", userID,
		"first_name", profile.FirstName,
		"last_name", profile.LastName,
	)

	return &profile, nil
}

// GetUserEmail получает email пользователя (альтернативный метод через Auth Service)
func (c *Client) GetUserEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	profile, err := c.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if profile == nil {
		return "", fmt.Errorf("user not found")
	}
	// Email может не быть в публичном профиле, нужен отдельный эндпоинт
	// Пока используем заглушку
	return fmt.Sprintf("user-%s@tripbuilder.com", userID.String()[:8]), nil
}

// GetUserFullName возвращает полное имя пользователя
func (c *Client) GetUserFullName(ctx context.Context, userID uuid.UUID) (string, error) {
	profile, err := c.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if profile == nil {
		return "Участник", nil
	}

	name := fmt.Sprintf("%s %s", profile.FirstName, profile.LastName)
	if name == " " {
		return "Участник", nil
	}
	return name, nil
}
