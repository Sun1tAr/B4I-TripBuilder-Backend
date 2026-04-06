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

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/client/event"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/client/media"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/consumer"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/repository/postgres"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/http/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/http/middleware"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	sharedMiddleware "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/middleware"
	sharedMigrate "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/migrate"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/retry"

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

	// Запуск миграций через shared утилиту
	if err := sharedMigrate.Run(db, sharedMigrate.Config{
		Schema:         cfg.Database.Schema,
		MigrationsPath: "file://cmd/server/migrations",
		Enabled:        cfg.MigrationEnabled,
	}, log); err != nil {
		log.Error("Ошибка применения миграций", "error", err)
		os.Exit(1)
	}

	// Инициализация репозиториев
	profileRepo := postgres.NewProfileRepository(db, cfg.Database.Schema, log)
	statsRepo := postgres.NewStatsRepository(db, cfg.Database.Schema, log)
	achievementRepo := postgres.NewAchievementRepository(db, cfg.Database.Schema, log)
	followRepo := postgres.NewFollowRepository(db, cfg.Database.Schema, log)

	// Инициализация клиентов других сервисов
	mediaClient := media.NewClient(cfg.MediaServiceURL, log)
	eventClient := event.NewClient(cfg.EventServiceURL, cfg.GetRetryConfig(), log)

	// Инициализация Kafka продюсера для пользовательских событий
	userProducer := kafka.NewUserProducer(cfg.KafkaBrokers, log)
	defer userProducer.Close()

	// Инициализация сервисов
	achievementService := service.NewAchievementService(achievementRepo, statsRepo, userProducer, log)
	statsService := service.NewStatsService(statsRepo, achievementService, log)
	profileService := service.NewProfileService(profileRepo, statsRepo, achievementRepo, userProducer, log) // передаём userProducer
	followService := service.NewFollowService(followRepo, profileRepo, log)

	// Инициализация хендлеров с передачей клиентов
	profileHandler := handlers.NewProfileHandler(profileService, mediaClient, eventClient, log)
	statsHandler := handlers.NewStatsHandler(statsService, log)
	achievementHandler := handlers.NewAchievementHandler(achievementService, log)
	followHandler := handlers.NewFollowHandler(followService, log)

	// Запуск RetryConsumer в фоне (если включено)
	if len(cfg.KafkaBrokers) > 0 && cfg.KafkaBrokers[0] != "" {
		log.Info("Создание RetryConsumer",
			"brokers", cfg.KafkaBrokers,
			"retry_topic", cfg.RetryTopic,
			"dlq_topic", cfg.DLQTopic,
		)
		retryConsumer := retry.NewRetryConsumer(cfg.GetRetryConfig(), log)
		go retryConsumer.Start(context.Background())
	}

	// Запуск Kafka consumer'ов для событий
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if len(cfg.KafkaBrokers) > 0 {
		groupID := "user-service"

		// user.created
		userCreatedConsumer := consumer.NewUserCreatedConsumer(
			cfg.KafkaBrokers,
			"user.created",
			groupID,
			profileService,
			log,
		)
		go userCreatedConsumer.Start(ctx)

		// route.created
		routeCreatedConsumer := consumer.NewRouteCreatedConsumer(
			cfg.KafkaBrokers,
			"route.created",
			groupID,
			statsService,
			log,
		)
		go routeCreatedConsumer.Start(ctx)

		// event.completed
		eventCompletedConsumer := consumer.NewEventCompletedConsumer(
			cfg.KafkaBrokers,
			"event.completed",
			groupID,
			statsService,
			log,
		)
		go eventCompletedConsumer.Start(ctx)

		// media.uploaded
		mediaUploadedConsumer := consumer.NewMediaUploadedConsumer(
			cfg.KafkaBrokers,
			"media.uploaded",
			groupID,
			statsService,
			log,
		)
		go mediaUploadedConsumer.Start(ctx)

		log.Info("Все Kafka consumers запущены")
	}

	// Роутер
	r := mux.NewRouter()

	// Публичные health-эндпоинты
	r.HandleFunc("/health", handlers.Health).Methods("GET")
	r.HandleFunc("/ready", handlers.Ready).Methods("GET")

	// API роутер с префиксом /api/v1/users
	api := r.PathPrefix("/api/v1/users").Subrouter()
	api.Use(middleware.Auth(log))

	// СНАЧАЛА специфичные маршруты (более конкретные)
	api.HandleFunc("/me", profileHandler.GetMyProfile).Methods("GET")
	api.HandleFunc("/me", profileHandler.UpdateProfile).Methods("PUT")
	api.HandleFunc("/me/avatar", profileHandler.UploadAvatar).Methods("POST")
	api.HandleFunc("/{id}/follow", followHandler.Follow).Methods("POST")
	api.HandleFunc("/{id}/follow", followHandler.Unfollow).Methods("DELETE")
	api.HandleFunc("/{id}/followers", followHandler.GetFollowers).Methods("GET")
	api.HandleFunc("/{id}/following", followHandler.GetFollowing).Methods("GET")

	// ПОТОМ общие маршруты (менее конкретные)
	api.HandleFunc("/{id}", profileHandler.GetPublicProfile).Methods("GET")
	api.HandleFunc("/{id}/stats", statsHandler.GetUserStats).Methods("GET")
	api.HandleFunc("/{id}/achievements", achievementHandler.GetUserAchievements).Methods("GET")
	api.HandleFunc("/{id}/events", profileHandler.GetUserEvents).Methods("GET")

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

	cancel() // останавливаем consumer'ов

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Ошибка при graceful shutdown", "error", err)
	}

	log.Info("Сервер остановлен")
}
