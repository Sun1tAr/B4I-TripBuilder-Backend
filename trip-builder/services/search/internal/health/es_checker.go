package health

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/elastic/go-elasticsearch/v8"
)

type ESWatcher struct {
	esClient      *elasticsearch.Client
	searchService *service.SearchService
	isAvailable   *atomic.Bool
	interval      time.Duration
	log           *logger.Logger
}

func NewESWatcher(esClient *elasticsearch.Client, searchService *service.SearchService, isAvailable *atomic.Bool, interval time.Duration, log *logger.Logger) *ESWatcher {
	return &ESWatcher{
		esClient:      esClient,
		searchService: searchService,
		isAvailable:   isAvailable,
		interval:      interval,
		log:           log,
	}
}

func (w *ESWatcher) Start(ctx context.Context) {
	w.log.Info("Запуск ES health watcher", "interval", w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("Остановка ES health watcher")
			return
		case <-ticker.C:
			w.checkAndRecover(ctx)
		}
	}
}

func (w *ESWatcher) checkAndRecover(ctx context.Context) {
	available := w.checkHealth(ctx)
	currentAvailable := w.isAvailable.Load()

	if available && !currentAvailable {
		w.log.Info("Elasticsearch восстановлен, запускаем полную реиндексацию")

		if err := w.searchService.FullReindexES(ctx); err != nil {
			w.log.Error("Ошибка реиндексации ES", "error", err)
			return
		}

		w.isAvailable.Store(true)
		w.log.Info("Elasticsearch полностью восстановлен, переключились на основной режим")
	}
}

func (w *ESWatcher) checkHealth(ctx context.Context) bool {
	res, err := w.esClient.Info()
	if err != nil {
		w.log.Debug("ES health check failed", "error", err)
		return false
	}
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK
}
