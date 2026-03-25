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

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/domain/repository/postgres"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/http/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/http/middleware"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/ratelimit"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/worker"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/health"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	sharedMiddleware "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/middleware"
	sharedMigrate "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/migrate"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
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
		Enabled:        true, // всегда включено, но можно добавить в конфиг
	}, log); err != nil {
		log.Error("Ошибка применения миграций", "error", err)
		os.Exit(1)
	}

	// Подключение к Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	// Проверка Redis при старте
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Error("Redis недоступен", "error", err)
		// Не выходим, rate limiting будет работать с ошибками
	} else {
		log.Info("Подключение к Redis установлено")
	}

	// Rate limiter
	rateLimiter := ratelimit.NewRateLimiter(redisClient, cfg.RateLimitLogin, cfg.RateLimitWindow, log)

	// Инициализация Kafka продюсера
	kafkaProducer := kafka.NewProducer(
		cfg.Kafka.Brokers,
		cfg.UserCreatedTopic, // <- берём из конфига
		log,
	)
	defer kafkaProducer.Close()

	// Инициализация репозиториев
	userRepo := postgres.NewUserRepository(db, cfg.Database.Schema)
	tokenRepo := postgres.NewRefreshTokenRepository(db, cfg.Database.Schema, log)

	// Инициализация сервиса
	authService := service.NewAuthService(cfg, userRepo, tokenRepo, log)

	// Инициализация воркера для очистки токенов
	tokenCleaner := worker.NewTokenCleaner(tokenRepo, log, cfg.TokenCleanupInterval)
	go tokenCleaner.Start(context.Background())

	// Создаём конфиг для health checker'а
	healthConfig := health.Config{
		RedisCheckInterval: cfg.RedisHealthInterval,
		KafkaCheckInterval: cfg.KafkaHealthInterval,
		KafkaDialTimeout:   cfg.KafkaDialTimeout,
		RedisPingTimeout:   cfg.RedisPingTimeout,
	}

	// Инициализация health checker для Kafka и Redis
	healthChecker := health.NewChecker(log, redisClient, cfg.Kafka.Brokers, healthConfig)
	go healthChecker.Start(context.Background())

	// Инициализация хендлеров
	authHandler := handlers.NewAuthHandler(authService, rateLimiter, kafkaProducer, log)

	// Роутер
	r := mux.NewRouter()

	// Публичные health-эндпоинты
	r.HandleFunc("/health", handlers.Health).Methods("GET")
	r.HandleFunc("/ready", handlers.Ready).Methods("GET")

	// API роутер с префиксом /api/v1/auth
	api := r.PathPrefix("/api/v1/auth").Subrouter()

	// Публичные маршруты (без аутентификации)
	api.HandleFunc("/register", authHandler.Register).Methods("POST")
	api.HandleFunc("/login", authHandler.Login).Methods("POST")
	api.HandleFunc("/refresh", authHandler.Refresh).Methods("POST")
	api.HandleFunc("/verify/{token}", authHandler.VerifyEmail).Methods("GET")
	api.HandleFunc("/reset-password", authHandler.RequestPasswordReset).Methods("POST")
	api.HandleFunc("/reset-password/{token}", authHandler.ResetPassword).Methods("POST")
	api.HandleFunc("/internal/users/by-email", authHandler.GetUserByEmail).Methods("GET")

	// Защищённые маршруты (требуют JWT)
	protected := api.PathPrefix("").Subrouter()
	protected.Use(middleware.Auth(cfg.JWTSecret))
	protected.HandleFunc("/logout", authHandler.Logout).Methods("POST")
	protected.HandleFunc("/me", authHandler.GetMe).Methods("GET")
	protected.HandleFunc("/validate", authHandler.Validate).Methods("GET")

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
