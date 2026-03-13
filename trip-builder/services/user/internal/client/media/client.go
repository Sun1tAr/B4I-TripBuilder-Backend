package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	log        *logger.Logger
}

func NewClient(baseURL string, log *logger.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: log,
	}
}

// UploadAvatar загружает аватар в Media Service и возвращает URL
func (c *Client) UploadAvatar(ctx context.Context, userID string, fileData []byte, filename string) (string, error) {
	// Подготавливаем multipart/form-data запрос
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return "", fmt.Errorf("failed to write file data: %w", err)
	}

	// Добавляем metadata (user_id)
	if err := writer.WriteField("user_id", userID); err != nil {
		return "", fmt.Errorf("failed to write user_id field: %w", err)
	}
	if err := writer.WriteField("purpose", "avatar"); err != nil {
		return "", fmt.Errorf("failed to write purpose field: %w", err)
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/media/upload", body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload to media service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("media service returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.URL, nil
}