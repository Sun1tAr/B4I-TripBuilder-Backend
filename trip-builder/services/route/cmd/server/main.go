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

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/domain/repository/postgres"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/http/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/http/middleware"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	sharedMiddleware "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/middleware"
	sharedMigrate "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/migrate"

	"github.com/go-chi/chi/v5"
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
	routeRepo := postgres.NewRouteRepository(db, cfg.Database.Schema, log)
	tagRepo := postgres.NewTagRepository(db, cfg.Database.Schema, log)
	likeRepo := postgres.NewLikeRepository(db, cfg.Database.Schema, log)

	// Инициализация Kafka продюсера
	kafkaProducer := kafka.NewRouteProducer(cfg.Kafka.Brokers, log)

	// Инициализация сервисов
	routeService := service.NewRouteService(routeRepo, tagRepo, likeRepo, kafkaProducer, log)

	// Инициализация хендлеров
	routeHandler := handlers.NewRouteHandler(routeService, log)

	// Роутер
	r := chi.NewRouter()

	// Глобальные middleware
	r.Use(sharedMiddleware.RequestID)
	r.Use(sharedMiddleware.Logging(log))
	r.Use(sharedMiddleware.Recover(log))

	// Публичные health-эндпоинты
	r.Get("/health", handlers.Health)
	r.Get("/ready", handlers.Ready)

	// API роутер с префиксом /api/v1/routes
	r.Route("/api/v1/routes", func(r chi.Router) {
		// ========== ПУБЛИЧНЫЕ МАРШРУТЫ (без авторизации) ==========
		r.Group(func(r chi.Router) {
			r.Get("/popular", routeHandler.GetPopularRoutes)
			r.Get("/tags", routeHandler.GetAllTags)
			r.Get("/tag/{tag}", routeHandler.GetRoutesByTag)
		})

		// ========== УСЛОВНО-ЗАКРЫТЫЕ (опциональная авторизация) ==========
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthOptional(log))
			r.Get("/{id}", routeHandler.GetRouteByID)
			r.Get("/", routeHandler.GetPublicRoutes)
		})

		// ========== ЗАКРЫТЫЕ МАРШРУТЫ (требуют авторизацию) ==========
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthRequired(log))

			r.Get("/my", routeHandler.GetMyRoutes)
			r.Post("/", routeHandler.CreateRoute)
			r.Post("/{id}/like", routeHandler.ToggleLike)
			r.Get("/{id}/likes", routeHandler.GetRouteLikes)
			r.Put("/{id}", routeHandler.UpdateRoute)
			r.Delete("/{id}", routeHandler.DeleteRoute)
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Ошибка при graceful shutdown", "error", err)
	}

	log.Info("Сервер остановлен")
}
