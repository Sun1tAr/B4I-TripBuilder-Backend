package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/health"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/middleware"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg := config.MustLoad()

	log := logger.New(logger.Options{
		Level:       cfg.GetLogLevel(),
		ServiceName: cfg.ServiceName,
		Format:      cfg.LogFormat,
	})

	log.Info("Запуск API Gateway", "port", cfg.HTTPPort)

	// Запускаем health checker в фоне
	checker := health.NewChecker(cfg, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go checker.Start(ctx)

	r := mux.NewRouter()

	// Публичные health-эндпоинты
	r.HandleFunc("/health", handlers.Health).Methods("GET")
	r.HandleFunc("/ready", handlers.Ready).Methods("GET")

	// API роутер с префиксом /api/v1
	api := r.PathPrefix("/api/v1").Subrouter()

	// ========== УСЛОВНО-ЗАКРЫТЫЕ МАРШРУТЫ (опциональная авторизация) ==========
	// Эти маршруты доступны всем, но если есть токен — он валидируется и передаётся
	conditional := api.PathPrefix("").Subrouter()
	conditional.Use(middleware.AuthOptional(cfg.JWTSecret, log))

	// Events - условно-закрытые (конкретные пути ДО общих)
	conditional.HandleFunc("/events/public", handlers.Proxy(cfg, "event")).Methods("GET")
	conditional.HandleFunc("/events/my", handlers.Proxy(cfg, "event")).Methods("GET")
	conditional.HandleFunc("/events/{id}", handlers.Proxy(cfg, "event")).Methods("GET")

	// Routes - условно-закрытые
	conditional.HandleFunc("/routes/{id}", handlers.Proxy(cfg, "route")).Methods("GET")

	// ========== ЗАКРЫТЫЕ МАРШРУТЫ (требуют авторизацию) ==========
	protected := api.PathPrefix("").Subrouter()
	protected.Use(middleware.AuthRequired(cfg.JWTSecret, log))

	// Auth
	protected.HandleFunc("/auth/logout", handlers.Proxy(cfg, "auth")).Methods("POST")
	protected.HandleFunc("/auth/me", handlers.Proxy(cfg, "auth")).Methods("GET")

	// Users - конкретные маршруты ДО общих
	protected.HandleFunc("/users/me", handlers.Proxy(cfg, "user")).Methods("GET")
	protected.HandleFunc("/users/me", handlers.Proxy(cfg, "user")).Methods("PUT")
	protected.HandleFunc("/users/me/avatar", handlers.Proxy(cfg, "user")).Methods("POST")
	protected.HandleFunc("/users/{id}/stats", handlers.Proxy(cfg, "user")).Methods("GET")
	protected.HandleFunc("/users/{id}/achievements", handlers.Proxy(cfg, "user")).Methods("GET")
	protected.HandleFunc("/users/{id}/events", handlers.Proxy(cfg, "user")).Methods("GET")
	protected.HandleFunc("/users/{id}/follow", handlers.Proxy(cfg, "user")).Methods("POST")
	protected.HandleFunc("/users/{id}/follow", handlers.Proxy(cfg, "user")).Methods("DELETE")
	protected.HandleFunc("/users/{id}", handlers.Proxy(cfg, "user")).Methods("GET")

	// Routes - конкретные маршруты ДО общих
	protected.HandleFunc("/routes/my", handlers.Proxy(cfg, "route")).Methods("GET")
	protected.HandleFunc("/routes/{id}/like", handlers.Proxy(cfg, "route")).Methods("POST")
	protected.HandleFunc("/routes/{id}/likes", handlers.Proxy(cfg, "route")).Methods("GET")
	protected.HandleFunc("/routes", handlers.Proxy(cfg, "route")).Methods("POST")
	protected.HandleFunc("/routes/{id}", handlers.Proxy(cfg, "route")).Methods("PUT")
	protected.HandleFunc("/routes/{id}", handlers.Proxy(cfg, "route")).Methods("DELETE")

	// Events - защищённые (POST, PUT, DELETE, управление участниками)
	protected.HandleFunc("/events/my", handlers.Proxy(cfg, "event")).Methods("GET")
	protected.HandleFunc("/events/invites", handlers.Proxy(cfg, "event")).Methods("GET")
	protected.HandleFunc("/events/invites/{invite_id}/accept", handlers.Proxy(cfg, "event")).Methods("POST")
	protected.HandleFunc("/events/invites/{invite_id}/decline", handlers.Proxy(cfg, "event")).Methods("POST")
	protected.HandleFunc("/events/{id}/invite", handlers.Proxy(cfg, "event")).Methods("POST")
	protected.HandleFunc("/events", handlers.Proxy(cfg, "event")).Methods("POST")
	protected.HandleFunc("/events/{id}", handlers.Proxy(cfg, "event")).Methods("PUT")
	protected.HandleFunc("/events/{id}", handlers.Proxy(cfg, "event")).Methods("DELETE")
	protected.HandleFunc("/events/{id}/join", handlers.Proxy(cfg, "event")).Methods("POST")
	protected.HandleFunc("/events/{id}/leave", handlers.Proxy(cfg, "event")).Methods("DELETE")
	protected.HandleFunc("/events/{id}/accept/{userId}", handlers.Proxy(cfg, "event")).Methods("POST")
	protected.HandleFunc("/events/{id}/decline/{userId}", handlers.Proxy(cfg, "event")).Methods("POST")
	protected.HandleFunc("/events/{id}/participants", handlers.Proxy(cfg, "event")).Methods("GET")
	protected.HandleFunc("/events/{id}/waypoints", handlers.Proxy(cfg, "event")).Methods("GET")
	protected.HandleFunc("/events/{id}/waypoints/{waypointId}/visit", handlers.Proxy(cfg, "event")).Methods("POST")
	protected.HandleFunc("/events/{id}/waypoints/{waypointId}/media", handlers.Proxy(cfg, "event")).Methods("POST")
	protected.HandleFunc("/events/{id}/waypoints/{waypointId}/comments", handlers.Proxy(cfg, "event")).Methods("POST")

	// Tasks
	protected.HandleFunc("/tasks", handlers.Proxy(cfg, "task")).Methods("GET")
	protected.HandleFunc("/tasks", handlers.Proxy(cfg, "task")).Methods("POST")
	protected.HandleFunc("/tasks/templates", handlers.Proxy(cfg, "task")).Methods("GET")
	protected.HandleFunc("/tasks/{id}/comments", handlers.Proxy(cfg, "task")).Methods("POST")
	protected.HandleFunc("/tasks/{id}", handlers.Proxy(cfg, "task")).Methods("GET")
	protected.HandleFunc("/tasks/{id}", handlers.Proxy(cfg, "task")).Methods("PUT")
	protected.HandleFunc("/tasks/{id}", handlers.Proxy(cfg, "task")).Methods("DELETE")

	// Chat
	protected.HandleFunc("/chat/events/{id}/messages", handlers.Proxy(cfg, "chat")).Methods("GET")
	protected.HandleFunc("/chat/events/{id}/messages", handlers.Proxy(cfg, "chat")).Methods("POST")
	protected.HandleFunc("/chat/events/{id}/unread", handlers.Proxy(cfg, "chat")).Methods("GET")

	// Media
	protected.HandleFunc("/media/upload", handlers.Proxy(cfg, "media")).Methods("POST")
	protected.HandleFunc("/media/upload/batch", handlers.Proxy(cfg, "media")).Methods("POST")
	protected.HandleFunc("/media/{id}", handlers.Proxy(cfg, "media")).Methods("GET")
	protected.HandleFunc("/media/{id}", handlers.Proxy(cfg, "media")).Methods("DELETE")
	protected.HandleFunc("/media/event/{eventId}", handlers.Proxy(cfg, "media")).Methods("GET")
	protected.HandleFunc("/media/waypoint/{waypointId}", handlers.Proxy(cfg, "media")).Methods("GET")
	protected.HandleFunc("/albums", handlers.Proxy(cfg, "media")).Methods("POST")
	protected.HandleFunc("/albums/{id}", handlers.Proxy(cfg, "media")).Methods("PUT")
	protected.HandleFunc("/albums/{id}/media", handlers.Proxy(cfg, "media")).Methods("POST")

	// Search
	protected.HandleFunc("/search/routes", handlers.Proxy(cfg, "search")).Methods("GET")
	protected.HandleFunc("/search/events", handlers.Proxy(cfg, "search")).Methods("GET")
	protected.HandleFunc("/search/users", handlers.Proxy(cfg, "search")).Methods("GET")
	protected.HandleFunc("/search/suggest", handlers.Proxy(cfg, "search")).Methods("GET")

	// Notifications
	protected.HandleFunc("/notifications", handlers.Proxy(cfg, "notification")).Methods("GET")
	protected.HandleFunc("/notifications/{id}/read", handlers.Proxy(cfg, "notification")).Methods("PUT")
	protected.HandleFunc("/notifications/read-all", handlers.Proxy(cfg, "notification")).Methods("PUT")
	protected.HandleFunc("/notifications/device", handlers.Proxy(cfg, "notification")).Methods("POST")
	protected.HandleFunc("/notifications/device/{token}", handlers.Proxy(cfg, "notification")).Methods("DELETE")

	// ========== ОТКРЫТЫЕ МАРШРУТЫ (без авторизации) ==========
	public := api.PathPrefix("").Subrouter()

	// Auth
	public.HandleFunc("/auth/register", handlers.Proxy(cfg, "auth")).Methods("POST")
	public.HandleFunc("/auth/login", handlers.Proxy(cfg, "auth")).Methods("POST")
	public.HandleFunc("/auth/refresh", handlers.Proxy(cfg, "auth")).Methods("POST")
	public.HandleFunc("/auth/reset-password", handlers.Proxy(cfg, "auth")).Methods("POST")
	public.HandleFunc("/auth/reset-password/{token}", handlers.Proxy(cfg, "auth")).Methods("POST")

	// Routes - открытые
	public.HandleFunc("/routes/popular", handlers.Proxy(cfg, "route")).Methods("GET")
	public.HandleFunc("/routes/tags", handlers.Proxy(cfg, "route")).Methods("GET")
	public.HandleFunc("/routes/tag/{tag}", handlers.Proxy(cfg, "route")).Methods("GET")
	public.HandleFunc("/routes", handlers.Proxy(cfg, "route")).Methods("GET")

	// Events - открытые (уже в conditional, но дубль для безопасности)
	// public.HandleFunc("/events/public", handlers.Proxy(cfg, "event")).Methods("GET") // уже в conditional

	// Middleware
	handler := middleware.Recovery(log)(r)
	handler = middleware.CORS(cfg)(handler)
	handler = middleware.Logging(log)(handler)
	handler = middleware.RateLimit(cfg)(handler)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

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

	cancel() // останавливаем health checker

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Ошибка при graceful shutdown", "error", err)
	}

	log.Info("Сервер остановлен")
}
