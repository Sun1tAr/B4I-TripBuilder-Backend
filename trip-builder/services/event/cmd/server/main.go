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
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/consumer"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/repository/postgres"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/http/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/http/middleware"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	sharedMiddleware "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/middleware"
	sharedMigrate "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/migrate"
	"github.com/redis/go-redis/v9"

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
	eventRepo := postgres.NewEventRepository(db, cfg.Database.Schema, log)
	participantRepo := postgres.NewParticipantRepository(db, cfg.Database.Schema, log)
	waypointRepo := postgres.NewWaypointRepository(db, cfg.Database.Schema, log)
	mediaRepo := postgres.NewMediaRepository(db, cfg.Database.Schema, log)
	commentRepo := postgres.NewCommentRepository(db, cfg.Database.Schema, log)

	// Инициализация Redis клиента для ссылок-приглашений
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	// Проверка Redis
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Error("Redis недоступен", "error", err)
		os.Exit(1)
	}
	log.Info("Подключение к Redis установлено")

	// Инициализация HTTP-клиентов
	routeClient := route.NewClient(cfg.RouteServiceURL, cfg.HTTPClientTimeout, log)
	authClient := auth.NewClient(cfg.AuthServiceURL, cfg.HTTPClientTimeout, log)

	// Инициализация Kafka продюсера
	eventProducer := kafka.NewEventProducer(cfg, log)

	// Инициализация сервисов
	eventService := service.NewEventService(
		eventRepo,
		participantRepo,
		waypointRepo,
		mediaRepo,
		commentRepo,
		routeClient,
		authClient,
		eventProducer,
		redisClient,
		log,
		cfg, // ← добавить cfg
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

	// Роутер
	r := mux.NewRouter()

	// Публичные health-эндпоинты
	r.HandleFunc("/health", handlers.Health).Methods("GET")
	r.HandleFunc("/ready", handlers.Ready).Methods("GET")

	// API роутер с префиксом /api/v1/events
	api := r.PathPrefix("/api/v1/events").Subrouter()

	// Публичные маршруты (регистрируем первыми)
	public := api.PathPrefix("").Subrouter()
	public.HandleFunc("/public", eventHandler.GetPublicEvents).Methods("GET")

	// Защищённые маршруты
	protected := api.PathPrefix("").Subrouter()
	protected.Use(middleware.Auth(log))

	// Конкретные маршруты ДО общих
	protected.HandleFunc("/my", eventHandler.GetMyEvents).Methods("GET")
	protected.HandleFunc("", eventHandler.CreateEvent).Methods("POST")
	protected.HandleFunc("/comments/{commentId}", eventHandler.UpdateComment).Methods("PUT")
	protected.HandleFunc("/comments/{commentId}", eventHandler.DeleteComment).Methods("DELETE")
	protected.HandleFunc("/media/{mediaId}", eventHandler.DeleteMedia).Methods("DELETE")

	// Приглашения
	protected.HandleFunc("/invites", eventHandler.GetInvites).Methods("GET")
	protected.HandleFunc("/invites/{invite_id}/accept", eventHandler.AcceptInvite).Methods("POST")
	protected.HandleFunc("/invites/{invite_id}/decline", eventHandler.DeclineInvite).Methods("POST")
	protected.HandleFunc("/{id}/invite", eventHandler.InviteParticipants).Methods("POST")

	protected.HandleFunc("/invites/new", eventHandler.CreateInviteLink).Methods("POST")
	protected.HandleFunc("/invite-link/{link_id}", eventHandler.GetEventByInviteLink).Methods("GET")

	protected.HandleFunc("/{id}/cover", eventHandler.UpdateEventCover).Methods("PUT")
	protected.HandleFunc("/{id}/join", eventHandler.JoinEvent).Methods("POST")
	protected.HandleFunc("/{id}/owner", eventHandler.CheckOwner).Methods("GET")
	protected.HandleFunc("/{id}/participant", eventHandler.CheckParticipant).Methods("GET")
	protected.HandleFunc("/{id}/leave", eventHandler.LeaveEvent).Methods("DELETE")
	protected.HandleFunc("/{id}/accept/{userId}", eventHandler.AcceptParticipant).Methods("POST")
	protected.HandleFunc("/{id}/decline/{userId}", eventHandler.DeclineParticipant).Methods("POST")
	protected.HandleFunc("/{id}/participants", eventHandler.GetParticipants).Methods("GET")
	protected.HandleFunc("/{id}/waypoints", eventHandler.GetWaypoints).Methods("GET")
	protected.HandleFunc("/{id}/waypoints/{waypointId}/visit", eventHandler.VisitWaypoint).Methods("POST")
	protected.HandleFunc("/{id}/waypoints/{waypointId}/media", eventHandler.AddMedia).Methods("POST")
	protected.HandleFunc("/{id}/waypoints/{waypointId}/media", eventHandler.GetMedia).Methods("GET")
	protected.HandleFunc("/{id}/waypoints/{waypointId}/comments", eventHandler.AddComment).Methods("POST")

	// Общие маршруты (с параметром) — САМЫЕ ПОСЛЕДНИЕ
	protected.HandleFunc("/{id}", eventHandler.GetEventByID).Methods("GET")
	protected.HandleFunc("/{id}", eventHandler.UpdateEvent).Methods("PUT")
	protected.HandleFunc("/{id}", eventHandler.DeleteEvent).Methods("DELETE")

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
