package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     interface {
		InfoContext(ctx context.Context, msg string, args ...any)
		ErrorContext(ctx context.Context, msg string, args ...any)
	}
}

func NewClient(baseURL string, timeout time.Duration, logger interface {
	InfoContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

func (c *Client) Do(ctx context.Context, method, path string, reqBody, respBody interface{}) error {
	url := c.baseURL + path
	var body io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if reqID, ok := ctx.Value("request_id").(string); ok {
		req.Header.Set("X-Request-ID", reqID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBodyBytes, _ := io.ReadAll(resp.Body)
		c.logger.ErrorContext(ctx, "http client error",
			"status", resp.Status,
			"body", string(respBodyBytes),
		)
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	if respBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}