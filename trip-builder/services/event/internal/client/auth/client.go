package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type UserInfo struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
}

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

// GetUserByEmail проверяет, существует ли пользователь с указанным email
func (c *Client) GetUserByEmail(ctx context.Context, email string) (*UserInfo, error) {
	url := fmt.Sprintf("%s/api/v1/auth/internal/users/by-email?email=%s", c.baseURL, email)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("AuthClient.GetUserByEmail: неожиданный статус", "status", resp.StatusCode, "email", email)
		return nil, fmt.Errorf("auth service returned %d", resp.StatusCode)
	}

	var result UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetUserEmail получает email пользователя по user_id
func (c *Client) GetUserEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	url := fmt.Sprintf("%s/api/v1/auth/internal/users/%s/email", c.baseURL, userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.log.Error("AuthClient.GetUserEmail: неожиданный статус", "status", resp.StatusCode, "user_id", userID)
		return "", fmt.Errorf("auth service returned %d", resp.StatusCode)
	}

	var result struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Email, nil
}
