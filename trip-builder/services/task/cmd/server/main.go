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

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/client/event"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/domain/repository/postgres"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/http/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/http/middleware"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/service"
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
	taskRepo := postgres.NewTaskRepository(db, cfg.Database.Schema, log)
	commentRepo := postgres.NewTaskCommentRepository(db, cfg.Database.Schema, log)
	templateRepo := postgres.NewTaskTemplateRepository(db, cfg.Database.Schema, log)

	// Инициализация клиента Event Service
	eventClient := event.NewClient(cfg.EventServiceURL, cfg.HTTPClientTimeout, log)

	// Инициализация Kafka продюсера
	taskProducer := kafka.NewTaskProducer(cfg.Kafka.Brokers, log)

	// Инициализация сервиса
	taskService := service.NewTaskService(taskRepo, commentRepo, templateRepo, eventClient, taskProducer, log)

	// Инициализация хендлеров
	taskHandler := handlers.NewTaskHandler(taskService, log)

	// Запуск Kafka consumer для событий
	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		// Consumer для event.created
		eventCreatedConsumer := kafka.NewEventConsumer(
			cfg.Kafka.Brokers,
			cfg.EventCreatedTopic,
			"task-service",
			taskService,
			log,
			cfg.GenerateTemplatesEnabled,
		)
		go eventCreatedConsumer.Start(context.Background())
		log.Info("Запущен consumer для топика", "topic", cfg.EventCreatedTopic, "generate_templates_enabled", cfg.GenerateTemplatesEnabled)

		// Consumer для event.updated
		eventUpdatedConsumer := kafka.NewEventConsumer(
			cfg.Kafka.Brokers,
			cfg.EventUpdatedTopic,
			"task-service",
			taskService,
			log,
			false,
		)
		go eventUpdatedConsumer.Start(context.Background())
		log.Info("Запущен consumer для топика", "topic", cfg.EventUpdatedTopic)

		// Consumer для event.cancelled
		eventCancelledConsumer := kafka.NewEventConsumer(
			cfg.Kafka.Brokers,
			cfg.EventCancelledTopic,
			"task-service",
			taskService,
			log,
			false,
		)
		go eventCancelledConsumer.Start(context.Background())
		log.Info("Запущен consumer для топика", "topic", cfg.EventCancelledTopic)
	}

	// Роутер
	r := mux.NewRouter()

	// Публичные health-эндпоинты
	r.HandleFunc("/health", handlers.Health).Methods("GET")
	r.HandleFunc("/ready", handlers.Ready).Methods("GET")

	// API роутер с префиксом /api/v1/tasks
	api := r.PathPrefix("/api/v1/tasks").Subrouter()
	api.Use(middleware.Auth(log))

	// Защищённые маршруты
	api.HandleFunc("", taskHandler.GetTasks).Methods("GET")
	api.HandleFunc("", taskHandler.CreateTask).Methods("POST")
	api.HandleFunc("/templates", taskHandler.GetTemplates).Methods("GET")
	api.HandleFunc("/{id}/approve", taskHandler.ApproveTask).Methods("POST")
	api.HandleFunc("/{id}/comments", taskHandler.AddComment).Methods("POST")
	api.HandleFunc("/{id}", taskHandler.GetTaskByID).Methods("GET")
	api.HandleFunc("/{id}", taskHandler.UpdateTask).Methods("PUT")
	api.HandleFunc("/{id}", taskHandler.DeleteTask).Methods("DELETE")

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
