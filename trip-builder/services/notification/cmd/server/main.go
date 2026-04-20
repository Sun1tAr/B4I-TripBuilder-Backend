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

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/repository/postgres"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/email"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/http/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/http/middleware"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/ratelimit"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/worker"
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

	log.Info("Запуск Notification Service",
		"http_port", cfg.HTTPPort,
		"log_level", cfg.LogLevel,
	)

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
	log.Info("Миграции успешно применены")

	// Подключение к Redis
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
	log.Info("Подключение к Redis установлено", "addr", cfg.Redis.Addr, "db", cfg.Redis.DB)

	// Инициализация репозиториев
	notificationRepo := postgres.NewNotificationRepository(db, cfg.Database.Schema, log)
	deviceRepo := postgres.NewDeviceRepository(db, cfg.Database.Schema, log)
	templateRepo := postgres.NewTemplateRepository(db, cfg.Database.Schema, log)

	log.Debug("Репозитории инициализированы")

	// Инициализация rate limiter для email
	emailRateLimiter := ratelimit.NewEmailRateLimiter(redisClient, cfg.EmailDailyLimit, log)
	log.Debug("Email rate limiter инициализирован",
		"daily_limit", cfg.EmailDailyLimit,
	)

	// Инициализация SMTP отправителя
	emailSender := email.NewSender(
		cfg.SMTPHost,
		cfg.SMTPPort,
		cfg.SMTPUsername,
		cfg.SMTPPassword,
		cfg.SMTPFrom,
		log,
	)
	log.Debug("SMTP отправитель инициализирован",
		"host", cfg.SMTPHost,
		"port", cfg.SMTPPort,
		"from", cfg.SMTPFrom,
	)

	// Инициализация шаблонизатора
	templateEngine := email.NewTemplateEngine(templateRepo, log)
	log.Debug("Шаблонизатор инициализирован")

	// Инициализация сервиса
	notificationService := service.NewNotificationService(
		notificationRepo,
		deviceRepo,
		emailSender,
		templateEngine,
		emailRateLimiter,
		log,
		cfg,
	)
	log.Debug("Notification service инициализирован")

	// Инициализация хендлеров
	notificationHandler := handlers.NewNotificationHandler(notificationService, log)
	log.Debug("HTTP хендлеры инициализированы")

	// Создаем контекст для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запуск воркера для очистки старых уведомлений
	cleanupWorker := worker.NewCleanupWorker(
		notificationRepo,
		deviceRepo,
		log,
		cfg.CleanupInterval,
		cfg.NotificationRetentionDays,
	)
	go cleanupWorker.Start(ctx)
	log.Info("Cleanup worker запущен",
		"interval", cfg.CleanupInterval,
		"retention_days", cfg.NotificationRetentionDays,
	)

	// Запуск Kafka консьюмеров
	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		groupID := "notification-service"

		log.Info("Запуск Kafka консьюмеров",
			"brokers", cfg.Kafka.Brokers,
			"group_id", groupID,
		)

		// Consumer для user.welcome
		if cfg.UserWelcomeTopic != "" {
			welcomeConsumer := kafka.NewUserWelcomeConsumer(
				cfg.Kafka.Brokers,
				cfg.UserWelcomeTopic,
				groupID,
				notificationService,
				log,
			)
			go welcomeConsumer.Start(ctx)
			log.Info("Consumer запущен", "topic", cfg.UserWelcomeTopic)
		}

		// Consumer для event.invite
		if cfg.EventInviteTopic != "" {
			inviteConsumer := kafka.NewEventInviteConsumer(
				cfg.Kafka.Brokers,
				cfg.EventInviteTopic,
				groupID,
				notificationService,
				log,
			)
			go inviteConsumer.Start(ctx)
			log.Info("Consumer запущен", "topic", cfg.EventInviteTopic)
		}

		// Consumer для task.assigned
		if cfg.TaskAssignedTopic != "" {
			taskConsumer := kafka.NewTaskAssignedConsumer(
				cfg.Kafka.Brokers,
				cfg.TaskAssignedTopic,
				groupID,
				notificationService,
				log,
			)
			go taskConsumer.Start(ctx)
			log.Info("Consumer запущен", "topic", cfg.TaskAssignedTopic)
		}

		// Consumer для chat.message
		if cfg.ChatMessageTopic != "" {
			chatConsumer := kafka.NewChatMessageConsumer(
				cfg.Kafka.Brokers,
				cfg.ChatMessageTopic,
				groupID,
				notificationService,
				log,
			)
			go chatConsumer.Start(ctx)
			log.Info("Consumer запущен", "topic", cfg.ChatMessageTopic)
		}

		// Consumer для event.reminder
		if cfg.EventReminderTopic != "" {
			reminderConsumer := kafka.NewEventReminderConsumer(
				cfg.Kafka.Brokers,
				cfg.EventReminderTopic,
				groupID,
				notificationService,
				log,
			)
			go reminderConsumer.Start(ctx)
			log.Info("Consumer запущен", "topic", cfg.EventReminderTopic)
		}
	} else {
		log.Warn("Kafka брокеры не настроены, консьюмеры не запущены")
	}

	// Роутер
	r := mux.NewRouter()

	// Публичные health-эндпоинты
	r.HandleFunc("/health", handlers.Health).Methods("GET")
	r.HandleFunc("/ready", handlers.Ready).Methods("GET")

	// API роутер с префиксом /api/v1/notifications
	api := r.PathPrefix("/api/v1/notifications").Subrouter()
	api.Use(middleware.Auth(log))

	// Маршруты
	api.HandleFunc("", notificationHandler.GetNotifications).Methods("GET")
	api.HandleFunc("/unread", notificationHandler.GetUnreadCount).Methods("GET")
	api.HandleFunc("/{id}/read", notificationHandler.MarkAsRead).Methods("PUT")
	api.HandleFunc("/read-all", notificationHandler.MarkAllAsRead).Methods("PUT")
	api.HandleFunc("/device", notificationHandler.RegisterDevice).Methods("POST")
	api.HandleFunc("/device/{token}", notificationHandler.UnregisterDevice).Methods("DELETE")

	log.Debug("HTTP маршруты настроены",
		"routes", []string{
			"GET /notifications",
			"GET /notifications/unread",
			"PUT /notifications/{id}/read",
			"PUT /notifications/read-all",
			"POST /notifications/device",
			"DELETE /notifications/device/{token}",
		},
	)

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
		log.Info("HTTP сервер запущен", "port", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Ошибка запуска сервера", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	log.Info("Получен сигнал завершения, начинаем graceful shutdown...")

	cancel() // останавливаем консьюмеры и воркеры

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Ошибка при graceful shutdown HTTP сервера", "error", err)
	}

	log.Info("Сервер остановлен")
}
