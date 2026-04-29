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

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/client/auth"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/client/route"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/client/user"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/consumer"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/repository/postgres"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/http/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/http/middleware"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/metrics"
	sharedMiddleware "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/middleware"
	sharedMigrate "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/migrate"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

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
	eventRepo := postgres.NewEventRepository(db, cfg.Database.Schema, log)
	participantRepo := postgres.NewParticipantRepository(db, cfg.Database.Schema, log)
	waypointRepo := postgres.NewWaypointRepository(db, cfg.Database.Schema, log)
	mediaRepo := postgres.NewMediaRepository(db, cfg.Database.Schema, log)
	commentRepo := postgres.NewCommentRepository(db, cfg.Database.Schema, log)
	transferRepo := postgres.NewEventTransferRepository(db, cfg.Database.Schema, log)

	// Инициализация Redis клиента
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Error("Redis недоступен", "error", err)
		os.Exit(1)
	}
	log.Info("Подключение к Redis установлено")

	// Инициализация HTTP-клиентов
	routeClient := route.NewClient(cfg.RouteServiceURL, cfg.HTTPClientTimeout, log)
	authClient := auth.NewClient(cfg.AuthServiceURL, cfg.HTTPClientTimeout, log)
	userClient := user.NewClient(cfg.UserServiceURL, cfg.HTTPClientTimeout, log)

	// Инициализация Kafka продюсера
	eventProducer := kafka.NewEventProducer(cfg, log)

	// Инициализация сервисов
	eventService := service.NewEventService(
		eventRepo,
		participantRepo,
		waypointRepo,
		mediaRepo,
		commentRepo,
		transferRepo,
		routeClient,
		authClient,
		userClient,
		eventProducer,
		redisClient,
		log,
		cfg,
	)

	// Запуск фонового воркера для обновления статусов поездок
	go eventService.StartStatusUpdater(context.Background(), cfg.StatusUpdateInterval)

	// Инициализация хендлеров
	eventHandler := handlers.NewEventHandler(eventService, log)

	// Запуск Kafka consumer для user.created
	if len(cfg.Kafka.Brokers) > 0 {
		userCreatedConsumer := consumer.NewUserCreatedConsumer(
			cfg.Kafka.Brokers,
			cfg.UserCreatedTopic,
			"event-service",
			participantRepo,
			log,
		)
		go userCreatedConsumer.Start(context.Background())
		log.Info("Запущен consumer для топика user.created")
	}

	// ========== РОУТЕР ==========
	r := chi.NewRouter()

	// Глобальные middleware
	r.Use(sharedMiddleware.RequestID)
	r.Use(sharedMiddleware.ChiMetricsMiddleware(cfg.ServiceName))
	r.Use(sharedMiddleware.Logging(log))
	r.Use(sharedMiddleware.Recover(log))

	// Публичные health-эндпоинты
	r.Get("/health", handlers.Health)
	r.Get("/ready", handlers.Ready)
	r.Handle("/metrics", metrics.Handler())

	// API роутер с префиксом /api/v1/events
	r.Route("/api/v1/events", func(r chi.Router) {
		// ========== ПУБЛИЧНЫЕ МАРШРУТЫ ==========
		r.Group(func(r chi.Router) {
			r.Get("/public", eventHandler.GetPublicEvents)
			r.Get("/invite-link/{link_id}", eventHandler.GetEventByInviteLink)
		})

		// ========== УСЛОВНО-ЗАКРЫТЫЕ (опциональная авторизация) ==========
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthOptional(log))
			r.Get("/{id}", eventHandler.GetEventByID)
		})

		// ========== ЗАКРЫТЫЕ МАРШРУТЫ (требуют авторизацию) ==========
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthRequired(log))

			r.Get("/my", eventHandler.GetMyEvents)

			// CRUD
			r.Post("/", eventHandler.CreateEvent)
			r.Put("/{id}", eventHandler.UpdateEvent)
			r.Delete("/{id}", eventHandler.DeleteEvent)

			// Комментарии
			r.Put("/comments/{commentId}", eventHandler.UpdateComment)
			r.Delete("/comments/{commentId}", eventHandler.DeleteComment)

			// Медиа
			r.Delete("/media/{mediaId}", eventHandler.DeleteMedia)

			// Приглашения
			r.Get("/invites", eventHandler.GetInvites)
			r.Post("/invites/{invite_id}/accept", eventHandler.AcceptInvite)
			r.Post("/invites/{invite_id}/decline", eventHandler.DeclineInvite)
			r.Post("/invites/new", eventHandler.CreateInviteLink)

			// Участники
			r.Post("/{id}/invite", eventHandler.InviteParticipants)
			r.Post("/{id}/join", eventHandler.JoinEvent)
			r.Delete("/{id}/leave", eventHandler.LeaveEvent)
			r.Post("/{id}/accept/{userId}", eventHandler.AcceptParticipant)
			r.Post("/{id}/decline/{userId}", eventHandler.DeclineParticipant)
			r.Get("/{id}/participants", eventHandler.GetParticipants)
			r.Get("/{id}/owner", eventHandler.CheckOwner)
			r.Get("/{id}/participant", eventHandler.CheckParticipant)

			// Точки маршрута
			r.Get("/{id}/waypoints", eventHandler.GetWaypoints)
			r.Post("/{id}/waypoints/{waypointId}/visit", eventHandler.VisitWaypoint)
			r.Post("/{id}/waypoints/{waypointId}/media", eventHandler.AddMedia)
			r.Get("/{id}/waypoints/{waypointId}/media", eventHandler.GetMedia)
			r.Post("/{id}/waypoints/{waypointId}/comments", eventHandler.AddComment)

			// Обложка
			r.Put("/{id}/cover", eventHandler.UpdateEventCover)
		})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      r,
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
