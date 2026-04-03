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
	wsHandler := websocket.NewWSHandler(hub, chatService, log, cfg.AllowedOrigins)

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

	// Роутер
	r := mux.NewRouter()

	// Публичные health-эндпоинты
	r.HandleFunc("/health", handlers.Health).Methods("GET")
	r.HandleFunc("/ready", handlers.Ready).Methods("GET")

	// API роутер с префиксом /api/v1/chat
	api := r.PathPrefix("/api/v1/chat").Subrouter()
	api.Use(middleware.Auth(log))

	chatHandler := handlers.NewChatHandler(chatService, log)

	// REST маршруты
	api.HandleFunc("/events/{id}/messages", chatHandler.GetMessages).Methods("GET")
	api.HandleFunc("/events/{id}/messages", chatHandler.SendMessage).Methods("POST")
	api.HandleFunc("/events/{id}/unread", chatHandler.GetUnreadCount).Methods("GET")
	api.HandleFunc("/messages/{id}", chatHandler.UpdateMessage).Methods("PUT")
	api.HandleFunc("/messages/{id}", chatHandler.DeleteMessage).Methods("DELETE")

	// WebSocket маршрут (тоже внутри /api/v1/chat)
	api.HandleFunc("/ws/events/{event_id}", wsHandler.ServeWS).Methods("GET")

	// Общие middleware
	handler := sharedMiddleware.RequestID(r)
	handler = sharedMiddleware.Logging(log)(handler)
	handler = sharedMiddleware.Recover(log)(handler)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
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
