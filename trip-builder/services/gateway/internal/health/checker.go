package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type Checker struct {
	cfg    *config.Config
	log    *logger.Logger
	client *http.Client
}

func NewChecker(cfg *config.Config, log *logger.Logger) *Checker {
	return &Checker{
		cfg: cfg,
		log: log,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Checker) Start(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.HealthCheckInterval)
	defer ticker.Stop()

	// Сразу выполняем первую проверку
	c.checkAllServices(ctx)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Остановка health checker")
			return
		case <-ticker.C:
			c.checkAllServices(ctx)
		}
	}
}

func (c *Checker) checkAllServices(ctx context.Context) {
	var wg sync.WaitGroup
	services := map[string]string{
		"auth":         c.cfg.Services.AuthURL,
		"user":         c.cfg.Services.UserURL,
		"route":        c.cfg.Services.RouteURL,
		"event":        c.cfg.Services.EventURL,
		"task":         c.cfg.Services.TaskURL,
		"chat":         c.cfg.Services.ChatURL,
		"media":        c.cfg.Services.MediaURL,
		"search":       c.cfg.Services.SearchURL,
		"notification": c.cfg.Services.NotificationURL,
	}

	for name, baseURL := range services {
		wg.Add(1)
		go func(serviceName, url string) {
			defer wg.Done()
			c.checkService(ctx, serviceName, url)
		}(name, baseURL)
	}

	wg.Wait()
}

func (c *Checker) checkService(ctx context.Context, name, baseURL string) {
	healthURL := baseURL + "/health"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		c.log.Error("Ошибка создания запроса health check",
			"service", name,
			"error", err,
		)
		return
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Error("Сервис недоступен",
			"service", name,
			"error", err,
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		c.log.Debug("Сервис доступен",
			"service", name,
			"status", resp.Status,
		)
	} else {
		c.log.Error("Сервис вернул неожиданный статус",
			"service", name,
			"status", resp.Status,
		)
	}
}