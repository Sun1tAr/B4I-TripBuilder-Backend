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

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/repository/postgres"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/http/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/http/middleware"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	sharedMiddleware "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/middleware"
	sharedMigrate "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/migrate"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS // всё ещё нужно для embed, но в коде не используется напрямую

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
		MigrationsPath: "file://cmd/server/migrations", // путь относительно запуска
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

	// Инициализация сервисов
	profileService := service.NewProfileService(profileRepo, statsRepo, achievementRepo, log)
	statsService := service.NewStatsService(statsRepo, log)
	achievementService := service.NewAchievementService(achievementRepo, log)
	followService := service.NewFollowService(followRepo, profileRepo, log)

	// Инициализация хендлеров
	profileHandler := handlers.NewProfileHandler(profileService, log)
	statsHandler := handlers.NewStatsHandler(statsService, log)
	achievementHandler := handlers.NewAchievementHandler(achievementService, log)
	followHandler := handlers.NewFollowHandler(followService, log)

	// Роутер
	r := mux.NewRouter()

	// Публичные health-эндпоинты
	r.HandleFunc("/health", handlers.Health).Methods("GET")
	r.HandleFunc("/ready", handlers.Ready).Methods("GET")

	// API роутер с префиксом /api/v1/users
	api := r.PathPrefix("/api/v1/users").Subrouter()

	// Публичные маршруты
	api.HandleFunc("/{id}", profileHandler.GetPublicProfile).Methods("GET")
	api.HandleFunc("/{id}/stats", statsHandler.GetUserStats).Methods("GET")
	api.HandleFunc("/{id}/achievements", achievementHandler.GetUserAchievements).Methods("GET")
	api.HandleFunc("/{id}/events", profileHandler.GetUserEvents).Methods("GET")

	// Защищённые маршруты
	protected := api.PathPrefix("").Subrouter()
	protected.Use(middleware.Auth(cfg.JWTSecret))

	protected.HandleFunc("/me", profileHandler.GetMyProfile).Methods("GET")
	protected.HandleFunc("/me", profileHandler.UpdateProfile).Methods("PUT")
	protected.HandleFunc("/me/avatar", profileHandler.UploadAvatar).Methods("POST")
	protected.HandleFunc("/{id}/follow", followHandler.Follow).Methods("POST")
	protected.HandleFunc("/{id}/follow", followHandler.Unfollow).Methods("DELETE")
	protected.HandleFunc("/{id}/followers", followHandler.GetFollowers).Methods("GET")
	protected.HandleFunc("/{id}/following", followHandler.GetFollowing).Methods("GET")

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Ошибка при graceful shutdown", "error", err)
	}

	log.Info("Сервер остановлен")
}