package main

import (
	"context"
	"database/sql"
	"embed"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/client/event"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/domain/repository/postgres"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/http/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/http/middleware"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/redis"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/websocket"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	sharedMiddleware "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/middleware"
	sharedMigrate "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/migrate"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	_ = godotenv.Load()

	cfg := config.MustLoad()

	log := logger.New(logger.Options{
		Level:       cfg.GetLogLevel(),
		ServiceName: cfg.ServiceName,
		Format:      cfg.LogFormat,
	})

	log.Info("Запуск Chat Service", "http_port", cfg.HTTPPort, "ws_port", cfg.WSPort)

	// Подключение к БД
	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		log.Error("Ошибка подключения к БД", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Error("БД недоступна", "error", err)
		os.Exit(1)
	}
	log.Info("Подключение к БД установлено", "schema", cfg.Database.Schema)

	// Запуск миграций
	if err := sharedMigrate.Run(db, sharedMigrate.Config{
		Schema:         cfg.Database.Schema,
		MigrationsPath: "file://cmd/server/migrations",
		Enabled:        cfg.MigrationEnabled,
	}, log); err != nil {
		log.Error("Ошибка применения миграций", "error", err)
		os.Exit(1)
	}

	// Инициализация репозиториев
	msgRepo := postgres.NewMessageRepository(db, cfg.Database.Schema, log)
	receiptRepo := postgres.NewReadReceiptRepository(db, cfg.Database.Schema, log)

	// Инициализация клиента Event Service
	eventClient := event.NewClient(cfg.EventServiceURL, cfg.HTTPClientTimeout, log)

	// Инициализация Redis Pub/Sub
	redisPubSub := redis.NewPubSub(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, log)

	// Инициализация Kafka producer (пока не используется, но оставляем)
	_ = kafka.NewChatProducer(cfg.Kafka.Brokers, log)

	// Инициализация сервиса
	chatService := service.NewChatService(msgRepo, receiptRepo, eventClient, redisPubSub, log)

	// Инициализация WebSocket Hub
	hub := websocket.NewHub()
	go hub.Run()

	// Инициализация WebSocket handler
	wsHandler := websocket.NewWSHandler(hub, chatService, redisPubSub, log, cfg.AllowedOrigins)

	// Запуск Kafka consumer для системных сообщений
	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		consumers := []struct {
			topic string
			name  string
		}{
			{topic: cfg.WaypointVisitedTopic, name: "waypoint.visited"},
			{topic: cfg.TaskCompletedTopic, name: "task.completed"},
			{topic: cfg.MediaUploadedTopic, name: "media.uploaded"},
			{topic: cfg.ParticipantJoinedTopic, name: "event.participant_joined"},
		}

		for _, c := range consumers {
			if c.topic == "" {
				log.Warn("Топик не задан, пропускаем", "name", c.name)
				continue
			}
			consumer := kafka.NewSystemEventConsumer(
				cfg.Kafka.Brokers,
				c.topic,
				"chat-service",
				chatService,
				log,
			)
			go consumer.Start(context.Background())
			log.Info("Запущен consumer для топика", "topic", c.topic)
		}
	}

	// ========== HTTP СЕРВЕР (REST API) ==========
	httpRouter := mux.NewRouter()

	// Публичные health-эндпоинты
	httpRouter.HandleFunc("/health", handlers.Health).Methods("GET")
	httpRouter.HandleFunc("/ready", handlers.Ready).Methods("GET")

	// API роутер с префиксом /api/v1/chat
	httpAPI := httpRouter.PathPrefix("/api/v1/chat").Subrouter()
	httpAPI.Use(middleware.Auth(log))

	chatHandler := handlers.NewChatHandler(chatService, log)

	// REST маршруты
	httpAPI.HandleFunc("/events/{id}/messages", chatHandler.GetMessages).Methods("GET")
	httpAPI.HandleFunc("/events/{id}/messages", chatHandler.SendMessage).Methods("POST")
	httpAPI.HandleFunc("/events/{id}/unread", chatHandler.GetUnreadCount).Methods("GET")
	httpAPI.HandleFunc("/messages/{id}", chatHandler.UpdateMessage).Methods("PUT")
	httpAPI.HandleFunc("/messages/{id}", chatHandler.DeleteMessage).Methods("DELETE")

	// Middleware для HTTP сервера
	httpHandler := sharedMiddleware.RequestID(httpRouter)
	httpHandler = sharedMiddleware.Logging(log)(httpHandler)
	httpHandler = sharedMiddleware.Recover(log)(httpHandler)

	httpSrv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      httpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ========== WEBSOCKET СЕРВЕР ==========
	wsRouter := mux.NewRouter()

	// Публичные health-эндпоинты
	wsRouter.HandleFunc("/health", handlers.Health).Methods("GET")
	wsRouter.HandleFunc("/ready", handlers.Ready).Methods("GET")

	// API роутер с префиксом /api/v1/chat
	wsAPI := wsRouter.PathPrefix("/api/v1/chat").Subrouter()
	wsAPI.Use(middleware.Auth(log))

	// WebSocket маршрут
	wsAPI.HandleFunc("/ws/events/{event_id}", wsHandler.ServeWS).Methods("GET")

	// Middleware для WebSocket сервера
	wsHandlerMiddleware := sharedMiddleware.RequestID(wsRouter)
	wsHandlerMiddleware = sharedMiddleware.Logging(log)(wsHandlerMiddleware)
	wsHandlerMiddleware = sharedMiddleware.Recover(log)(wsHandlerMiddleware)

	wsSrv := &http.Server{
		Addr:         ":" + cfg.WSPort,
		Handler:      wsHandlerMiddleware,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ========== ЗАПУСК СЕРВЕРОВ ==========
	go func() {
		log.Info("HTTP сервер запущен", "port", cfg.HTTPPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Ошибка HTTP сервера", "error", err)
			os.Exit(1)
		}
	}()

	go func() {
		log.Info("WebSocket сервер запущен", "port", cfg.WSPort)
		if err := wsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Ошибка WebSocket сервера", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-done
	log.Info("Получен сигнал завершения, останавливаем серверы")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("Ошибка при остановке HTTP сервера", "error", err)
	}

	if err := wsSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("Ошибка при остановке WebSocket сервера", "error", err)
	}

	log.Info("Серверы остановлены")
}
