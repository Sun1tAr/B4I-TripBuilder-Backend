package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/repository/elasticsearch"
	redisrepo "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/repository/redis"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/health"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/http/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/http/middleware"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	sharedMiddleware "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/middleware"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg := config.MustLoad()

	log := logger.New(logger.Options{
		Level:       cfg.GetLogLevel(),
		ServiceName: cfg.ServiceName,
		Format:      cfg.LogFormat,
	})

	log.Info("Запуск Search Service", "port", cfg.HTTPPort, "mode", cfg.SearchMode)

	var searchService service.SearchServiceInterface

	switch cfg.SearchMode {
	case "proxy":
		log.Info("Запуск в режиме PROXY (прокси к сервисам, без ES/Redis)")
		searchService = service.NewProxySearchService(cfg, log)

	default:
		log.Info("Запуск в режиме FULL (Elasticsearch + Redis с fallback)")
		searchService = setupFullMode(cfg, log)
	}

	// ========== HTTP СЕРВЕР ==========
	r := mux.NewRouter()
	r.HandleFunc("/health", handlers.Health).Methods("GET")
	r.HandleFunc("/ready", handlers.Ready).Methods("GET")

	api := r.PathPrefix("/api/v1/search").Subrouter()
	searchHandler := handlers.NewSearchHandler(searchService, log)

	// Публичные эндпоинты
	public := api.PathPrefix("").Subrouter()
	public.Use(middleware.AuthOptional(log))
	public.HandleFunc("/routes", searchHandler.SearchRoutes).Methods("GET")
	public.HandleFunc("/events", searchHandler.SearchEvents).Methods("GET")
	public.HandleFunc("/suggest", searchHandler.Suggest).Methods("GET")

	// Защищённые эндпоинты
	protected := api.PathPrefix("").Subrouter()
	protected.Use(middleware.AuthRequired(log))
	protected.HandleFunc("/users", searchHandler.SearchUsers).Methods("GET")
	protected.HandleFunc("/sync", searchHandler.SyncFullReindex).Methods("POST")

	// Middleware
	handler := sharedMiddleware.RequestID(r)
	handler = sharedMiddleware.Logging(log)(handler)
	handler = sharedMiddleware.Recover(log)(handler)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Ошибка запуска сервера", "error", err)
			os.Exit(1)
		}
	}()

	log.Info("Сервер запущен", "port", cfg.HTTPPort)
	<-done
	log.Info("Получен сигнал завершения, останавливаемся")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Ошибка graceful shutdown", "error", err)
	}

	log.Info("Сервер остановлен")
}

func setupFullMode(cfg *config.Config, log *logger.Logger) service.SearchServiceInterface {
	// ========== ИНИЦИАЛИЗАЦИЯ REDIS (ВСЕГДА) ==========
	redisClient := redisrepo.NewClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, log)
	if err := redisClient.Ping(context.Background()); err != nil {
		log.Error("Ошибка подключения к Redis", "error", err)
		os.Exit(1)
	}
	log.Info("Подключение к Redis установлено")

	// Redis репозитории (всегда создаются)
	routeRedis := redisrepo.NewRouteRepository(redisClient, log)
	eventRedis := redisrepo.NewEventRepository(redisClient, log)
	userRedis := redisrepo.NewUserRepository(redisClient, log)

	// ========== ИНИЦИАЛИЗАЦИЯ ELASTICSEARCH (ЕСЛИ ДОСТУПЕН) ==========
	var routeES repository.RouteRepository
	var eventES repository.EventRepository
	var userES repository.UserRepository
	var esAvailable atomic.Bool

	esClient, err := elasticsearch.NewClient(cfg, log)
	if err != nil {
		log.Warn("Elasticsearch недоступен, работаем только с Redis", "error", err)
		esAvailable.Store(false)
	} else {
		routeES = elasticsearch.NewRouteRepository(esClient, log, cfg.MaxResultWindow)
		eventES = elasticsearch.NewEventRepository(esClient, log, cfg.MaxResultWindow)
		userES = elasticsearch.NewUserRepository(esClient, log, cfg.MaxResultWindow)

		// Создаём индексы
		ctx := context.Background()
		if err := routeES.EnsureIndex(ctx); err != nil {
			log.Error("Ошибка создания индекса routes", "error", err)
		}
		if err := eventES.EnsureIndex(ctx); err != nil {
			log.Error("Ошибка создания индекса events", "error", err)
		}
		if err := userES.EnsureIndex(ctx); err != nil {
			log.Error("Ошибка создания индекса users", "error", err)
		}

		esAvailable.Store(true)
		log.Info("Elasticsearch доступен, работаем в основном режиме")
	}

	// ========== СЕРВИС (ВСЕГДА ПОЛУЧАЕТ ОБА НАБОРА РЕПОЗИТОРИЕВ) ==========
	searchSvc := service.NewSearchService(
		routeES, eventES, userES,
		routeRedis, eventRedis, userRedis,
		&esAvailable,
		cfg, log,
	)

	// ========== ES WATCHER (ДЛЯ ВОССТАНОВЛЕНИЯ) ==========
	if esClient != nil && cfg.ESHealthCheckInterval > 0 {
		watcher := health.NewESWatcher(
			esClient.GetClient(),
			searchSvc,
			&esAvailable,
			cfg.ESHealthCheckInterval,
			log,
		)
		go watcher.Start(context.Background())
	}

	// ========== KAFKA CONSUMERS ==========
	if len(cfg.KafkaBrokers) > 0 && cfg.KafkaBrokers[0] != "" {
		groupID := "search-service"

		topics := []string{
			cfg.RouteCreatedTopic, cfg.RouteUpdatedTopic, cfg.RouteDeletedTopic,
			cfg.EventCreatedTopic, cfg.EventUpdatedTopic, cfg.EventDeletedTopic,
			cfg.UserCreatedTopic, cfg.UserUpdatedTopic,
		}

		for _, topic := range topics {
			if topic == "" {
				continue
			}
			consumer := kafka.NewEventConsumer(cfg.KafkaBrokers, topic, groupID, searchSvc, log)
			go consumer.Start(context.Background())
			log.Info("Запущен consumer", "topic", topic)
		}
	}

	return searchSvc
}
