package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/repository/elasticsearch"
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

	log.Info("Запуск Search Service", "port", cfg.HTTPPort)

	// Инициализация Elasticsearch клиента
	esClient, err := elasticsearch.NewESClient(cfg, log)
	if err != nil {
		log.Error("Ошибка инициализации Elasticsearch", "error", err)
		os.Exit(1)
	}

	// Инициализация репозиториев
	routeRepo := elasticsearch.NewRouteRepository(esClient, log, cfg.MaxResultWindow)
	eventRepo := elasticsearch.NewEventRepository(esClient, log, cfg.MaxResultWindow)
	userRepo := elasticsearch.NewUserRepository(esClient, log, cfg.MaxResultWindow)

	// Создание индексов
	ctx := context.Background()
	if err := routeRepo.EnsureIndex(ctx); err != nil {
		log.Error("Ошибка создания индекса routes", "error", err)
		os.Exit(1)
	}
	if err := eventRepo.EnsureIndex(ctx); err != nil {
		log.Error("Ошибка создания индекса events", "error", err)
		os.Exit(1)
	}
	if err := userRepo.EnsureIndex(ctx); err != nil {
		log.Error("Ошибка создания индекса users", "error", err)
		os.Exit(1)
	}

	// Инициализация сервиса
	searchService := service.NewSearchService(routeRepo, eventRepo, userRepo, cfg, log)

	// Запуск периодической переиндексации
	go searchService.StartPeriodicReindex(ctx, cfg.InitialSyncInterval)

	// Инициализация Kafka consumer'ов
	if len(cfg.KafkaBrokers) > 0 && cfg.KafkaBrokers[0] != "" {
		groupID := "search-service"

		// Consumer для маршрутов
		routeConsumer := kafka.NewEventConsumer(cfg.KafkaBrokers, cfg.RouteCreatedTopic, groupID, searchService, log)
		go routeConsumer.Start(ctx)

		routeUpdateConsumer := kafka.NewEventConsumer(cfg.KafkaBrokers, cfg.RouteUpdatedTopic, groupID, searchService, log)
		go routeUpdateConsumer.Start(ctx)

		routeDeleteConsumer := kafka.NewEventConsumer(cfg.KafkaBrokers, cfg.RouteDeletedTopic, groupID, searchService, log)
		go routeDeleteConsumer.Start(ctx)

		// Consumer для событий
		eventCreatedConsumer := kafka.NewEventConsumer(cfg.KafkaBrokers, cfg.EventCreatedTopic, groupID, searchService, log)
		go eventCreatedConsumer.Start(ctx)

		eventUpdatedConsumer := kafka.NewEventConsumer(cfg.KafkaBrokers, cfg.EventUpdatedTopic, groupID, searchService, log)
		go eventUpdatedConsumer.Start(ctx)

		eventDeletedConsumer := kafka.NewEventConsumer(cfg.KafkaBrokers, cfg.EventDeletedTopic, groupID, searchService, log)
		go eventDeletedConsumer.Start(ctx)

		// Consumer для пользователей
		userCreatedConsumer := kafka.NewEventConsumer(cfg.KafkaBrokers, cfg.UserCreatedTopic, groupID, searchService, log)
		go userCreatedConsumer.Start(ctx)

		userUpdatedConsumer := kafka.NewEventConsumer(cfg.KafkaBrokers, cfg.UserUpdatedTopic, groupID, searchService, log)
		go userUpdatedConsumer.Start(ctx)

		log.Info("Все Kafka consumers запущены")
	}

	// Роутер
	r := mux.NewRouter()

	// Публичные health-эндпоинты
	r.HandleFunc("/health", handlers.Health).Methods("GET")
	r.HandleFunc("/ready", handlers.Ready).Methods("GET")

	// API роутер с префиксом /api/v1/search
	api := r.PathPrefix("/api/v1/search").Subrouter()

	searchHandler := handlers.NewSearchHandler(searchService, log)

	// Публичные эндпоинты (с условной авторизацией)
	public := api.PathPrefix("").Subrouter()
	public.Use(middleware.AuthOptional(log))
	public.HandleFunc("/routes", searchHandler.SearchRoutes).Methods("GET")
	public.HandleFunc("/events", searchHandler.SearchEvents).Methods("GET")
	public.HandleFunc("/suggest", searchHandler.Suggest).Methods("GET")

	// Защищённые эндпоинты (только для авторизованных)
	protected := api.PathPrefix("").Subrouter()
	protected.Use(middleware.AuthRequired(log))
	protected.HandleFunc("/users", searchHandler.SearchUsers).Methods("GET")
	protected.HandleFunc("/sync", searchHandler.SyncFullReindex).Methods("POST")

	// Общие middleware
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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Ошибка при graceful shutdown", "error", err)
	}

	log.Info("Сервер остановлен")
}
