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

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/domain/repository/postgres"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/http/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/http/middleware"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/minio"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/service"
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
	mediaRepo := postgres.NewMediaRepository(db, cfg.Database.Schema, log)

	// Инициализация MinIO клиента
	minioClient, err := minio.NewClient(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucket,
		cfg.MinIOUseSSL,
		log,
	)
	if err != nil {
		log.Error("Ошибка инициализации MinIO", "error", err)
		os.Exit(1)
	}
	defer minioClient.Close()

	// Инициализация Kafka продюсера
	mediaProducer := kafka.NewMediaProducer(cfg.Kafka.Brokers, log)
	defer mediaProducer.Close()

	// Инициализация сервиса
	mediaService := service.NewMediaService(
		mediaRepo,
		minioClient,
		mediaProducer,
		log,
		cfg.MaxImageSize,
		cfg.MaxVideoSize,
		cfg.PublicMediaURL,
	)

	// Инициализация хендлеров
	mediaHandler := handlers.NewMediaHandler(mediaService, log)

	// Роутер
	r := mux.NewRouter()

	// Публичные health-эндпоинты
	r.HandleFunc("/health", handlers.Health).Methods("GET")
	r.HandleFunc("/ready", handlers.Ready).Methods("GET")

	// API роутер с префиксом /api/v1/media
	api := r.PathPrefix("/api/v1/media").Subrouter()
	public := api.PathPrefix("").Subrouter()
	protected := api.PathPrefix("").Subrouter()
	protected.Use(middleware.Auth(log))

	public.HandleFunc("/waypoint/{waypointId}", mediaHandler.GetByWaypointID).Methods("GET")
	public.HandleFunc("/event/{eventId}", mediaHandler.GetByEventID).Methods("GET")
	protected.HandleFunc("/upload/batch", mediaHandler.UploadBatch).Methods("POST")
	protected.HandleFunc("/upload", mediaHandler.Upload).Methods("POST")
	public.HandleFunc("/{id}/file", mediaHandler.ServeFile).Methods("GET")
	public.HandleFunc("/{id}", mediaHandler.GetByID).Methods("GET")
	protected.HandleFunc("/{id}", mediaHandler.Delete).Methods("DELETE")

	// Общие middleware
	handler := sharedMiddleware.RequestID(r)
	handler = sharedMiddleware.Logging(log)(handler)
	handler = sharedMiddleware.Recover(log)(handler)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
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
