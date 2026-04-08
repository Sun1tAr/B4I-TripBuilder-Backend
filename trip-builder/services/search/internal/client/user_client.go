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

type UserClient struct {
	baseURL    string
	httpClient *http.Client
	log        *logger.Logger
}

func NewUserClient(baseURL string, timeout time.Duration, log *logger.Logger) *UserClient {
	return &UserClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
		log:        log,
	}
}

// SearchUsers выполняет поиск пользователей
func (c *UserClient) SearchUsers(ctx context.Context, req *models.UserSearchRequest) (*models.UserSearchResponse, error) {
	params := url.Values{}

	if req.Query != "" {
		params.Set("search", req.Query)
	}
	params.Set("page", strconv.Itoa(req.Page))
	params.Set("size", strconv.Itoa(req.Size))

	fullURL := fmt.Sprintf("%s/api/v1/users?%s", c.baseURL, params.Encode())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	// Поиск пользователей требует авторизации
	if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
		httpReq.Header.Set("X-User-ID", userID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user service returned %d: %s", resp.StatusCode, string(body))
	}

	var result models.UserSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
