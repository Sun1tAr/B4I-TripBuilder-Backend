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
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/metrics"
	sharedMiddleware "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/middleware"
	"github.com/go-chi/chi/v5"
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

	log.Info("Запуск API Gateway", "http_port", cfg.HTTPPort, "ws_port", cfg.WSPort)

	// Запускаем health checker в фоне
	checker := health.NewChecker(cfg, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go checker.Start(ctx)

	// ========== HTTP СЕРВЕР (REST API) ==========
	httpRouter := chi.NewRouter()

	// Глобальные middleware
	httpRouter.Use(sharedMiddleware.RequestID)
	httpRouter.Use(sharedMiddleware.ChiMetricsMiddleware(cfg.ServiceName))
	httpRouter.Use(middleware.Recovery(log))
	httpRouter.Use(middleware.CORS(cfg))
	httpRouter.Use(middleware.Logging(log))
	httpRouter.Use(middleware.RateLimit(cfg))

	// Health эндпоинты
	httpRouter.Get("/health", handlers.Health)
	httpRouter.Get("/ready", handlers.Ready)
	httpRouter.Handle("/metrics", metrics.Handler())

	// API v1
	httpRouter.Route("/api/v1", func(r chi.Router) {
		// ========== ПУБЛИЧНЫЕ МАРШРУТЫ (без авторизации) ==========
		r.Group(func(r chi.Router) {
			// Auth
			r.Post("/auth/register", handlers.Proxy(cfg, "auth"))
			r.Post("/auth/login", handlers.Proxy(cfg, "auth"))
			r.Post("/auth/refresh", handlers.Proxy(cfg, "auth"))
			r.Post("/auth/reset-password/{token}", handlers.Proxy(cfg, "auth"))
			r.Get("/auth/verify/{token}", handlers.Proxy(cfg, "auth"))

			// Routes
			r.Get("/routes/popular", handlers.Proxy(cfg, "route"))
			r.Get("/routes/tags", handlers.Proxy(cfg, "route"))
			r.Get("/routes/tag/{tag}", handlers.Proxy(cfg, "route"))

			// Events
			r.Get("/events/public", handlers.Proxy(cfg, "event"))
			r.Get("/events/invite-link/{link_id}", handlers.Proxy(cfg, "event"))

			// Users (публичные профили)
			r.Get("/users/{id}/stats", handlers.Proxy(cfg, "user"))
			r.Get("/users/{id}/achievements", handlers.Proxy(cfg, "user"))
			r.Get("/users/{id}/events", handlers.Proxy(cfg, "user"))
			r.Get("/users/{id}/followers", handlers.Proxy(cfg, "user"))
			r.Get("/users/{id}/following", handlers.Proxy(cfg, "user"))
			r.Get("/users/{id}", handlers.Proxy(cfg, "user"))

			// Media (публичные файлы)
			r.Get("/media/event/{eventId}", handlers.Proxy(cfg, "media"))
			r.Get("/media/waypoint/{waypointId}", handlers.Proxy(cfg, "media"))
			r.Get("/media/{id}/file", handlers.Proxy(cfg, "media"))
			r.Get("/media/{id}", handlers.Proxy(cfg, "media"))
		})

		// ========== УСЛОВНО-ЗАКРЫТЫЕ (опциональная авторизация) ==========
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthOptional(cfg.JWTSecret, log))

			r.Get("/routes", handlers.Proxy(cfg, "route"))
			r.Post("/auth/reset-password", handlers.Proxy(cfg, "auth"))

			r.Get("/events/my", handlers.Proxy(cfg, "event"))
			r.Get("/events/{id}", handlers.Proxy(cfg, "event"))

			// Search
			r.Get("/search/routes", handlers.Proxy(cfg, "search"))
			r.Get("/search/events", handlers.Proxy(cfg, "search"))
			r.Get("/search/suggest", handlers.Proxy(cfg, "search"))

			r.Get("/routes/{id}", handlers.Proxy(cfg, "route"))
		})

		// ========== ЗАКРЫТЫЕ МАРШРУТЫ (требуют авторизацию) ==========
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthRequired(cfg.JWTSecret, log))

			// Auth
			r.Post("/auth/logout", handlers.Proxy(cfg, "auth"))
			r.Get("/auth/me", handlers.Proxy(cfg, "auth"))

			// Users
			r.Get("/users/me", handlers.Proxy(cfg, "user"))
			r.Put("/users/me", handlers.Proxy(cfg, "user"))
			r.Post("/users/me/avatar", handlers.Proxy(cfg, "user"))
			r.Post("/users/{id}/follow", handlers.Proxy(cfg, "user"))
			r.Delete("/users/{id}/follow", handlers.Proxy(cfg, "user"))

			// Routes
			r.Post("/routes", handlers.Proxy(cfg, "route"))
			r.Get("/routes/my", handlers.Proxy(cfg, "route"))
			r.Get("/routes/liked", handlers.Proxy(cfg, "route"))
			r.Post("/routes/{id}/like", handlers.Proxy(cfg, "route"))
			r.Get("/routes/{id}/likes", handlers.Proxy(cfg, "route"))
			r.Put("/routes/{id}", handlers.Proxy(cfg, "route"))
			r.Delete("/routes/{id}", handlers.Proxy(cfg, "route"))

			// Events
			r.Get("/events/my", handlers.Proxy(cfg, "event"))
			r.Get("/events/invites", handlers.Proxy(cfg, "event"))
			r.Post("/events/invites/{invite_id}/accept", handlers.Proxy(cfg, "event"))
			r.Post("/events/invites/{invite_id}/decline", handlers.Proxy(cfg, "event"))
			r.Post("/events/invites/new", handlers.Proxy(cfg, "event"))
			r.Post("/events/{id}/invite", handlers.Proxy(cfg, "event"))
			r.Post("/events/{id}/join", handlers.Proxy(cfg, "event"))
			r.Delete("/events/{id}/leave", handlers.Proxy(cfg, "event"))
			r.Post("/events/{id}/accept/{userId}", handlers.Proxy(cfg, "event"))
			r.Post("/events/{id}/decline/{userId}", handlers.Proxy(cfg, "event"))
			r.Get("/events/{id}/participants", handlers.Proxy(cfg, "event"))
			r.Get("/events/{id}/waypoints", handlers.Proxy(cfg, "event"))
			r.Post("/events/{id}/waypoints/{waypointId}/visit", handlers.Proxy(cfg, "event"))
			r.Post("/events/{id}/waypoints/{waypointId}/media", handlers.Proxy(cfg, "event"))
			r.Post("/events/{id}/waypoints/{waypointId}/comments", handlers.Proxy(cfg, "event"))
			r.Put("/events/{id}/cover", handlers.Proxy(cfg, "event"))
			r.Post("/events", handlers.Proxy(cfg, "event"))
			r.Delete("/events/{id}", handlers.Proxy(cfg, "event"))
			r.Put("/events/{id}", handlers.Proxy(cfg, "event"))

			// Tasks
			r.Get("/tasks", handlers.Proxy(cfg, "task"))
			r.Post("/tasks", handlers.Proxy(cfg, "task"))
			r.Get("/tasks/templates", handlers.Proxy(cfg, "task"))
			r.Post("/tasks/{id}/approve", handlers.Proxy(cfg, "task"))
			r.Post("/tasks/{id}/comments", handlers.Proxy(cfg, "task"))
			r.Get("/tasks/{id}/comments", handlers.Proxy(cfg, "task"))
			r.Get("/tasks/{id}", handlers.Proxy(cfg, "task"))
			r.Put("/tasks/{id}", handlers.Proxy(cfg, "task"))
			r.Delete("/tasks/{id}", handlers.Proxy(cfg, "task"))

			// Chat (REST)
			r.Get("/chat/events/{id}/messages", handlers.Proxy(cfg, "chat"))
			r.Post("/chat/events/{id}/messages", handlers.Proxy(cfg, "chat"))
			r.Get("/chat/events/{id}/unread", handlers.Proxy(cfg, "chat"))
			r.Put("/chat/messages/{id}", handlers.Proxy(cfg, "chat"))

			// Media
			r.Post("/media/upload", handlers.Proxy(cfg, "media"))
			r.Post("/media/upload/batch", handlers.Proxy(cfg, "media"))
			r.Delete("/media/{id}", handlers.Proxy(cfg, "media"))

			// Search
			r.Get("/search/users", handlers.Proxy(cfg, "search"))

			// Notifications
			r.Get("/notifications", handlers.Proxy(cfg, "notification"))
			r.Put("/notifications/{id}/read", handlers.Proxy(cfg, "notification"))
			r.Put("/notifications/read-all", handlers.Proxy(cfg, "notification"))
			r.Post("/notifications/device", handlers.Proxy(cfg, "notification"))
			r.Delete("/notifications/device/{token}", handlers.Proxy(cfg, "notification"))
		})
	})

	httpSrv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      httpRouter,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// ========== WEBSOCKET СЕРВЕР ==========
	wsRouter := chi.NewRouter()
	wsRouter.Use(sharedMiddleware.RequestID)
	wsRouter.Use(sharedMiddleware.ChiMetricsMiddleware(cfg.ServiceName))
	wsRouter.Use(middleware.Recovery(log))
	wsRouter.Use(middleware.CORS(cfg))
	wsRouter.Use(middleware.Logging(log))

	wsRouter.Get("/health", handlers.Health)
	wsRouter.Get("/ready", handlers.Ready)

	wsRouter.Route("/api/v1", func(r chi.Router) {
		r.Get("/chat/ws/events/{event_id}", handlers.WebSocketProxy(cfg, "chat"))
	})

	wsSrv := &http.Server{
		Addr:         ":" + cfg.WSPort,
		Handler:      wsRouter,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// ========== ЗАПУСК СЕРВЕРОВ ==========
	go func() {
		log.Info("HTTP сервер запущен", "port", cfg.HTTPPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Ошибка HTTP сервера", "error", err)
			os.Exit(1)
		}
	}()

	go func() {
		log.Info("WebSocket сервер запущен", "port", cfg.WSPort)
		if err := wsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Ошибка WebSocket сервера", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-done
	log.Info("Получен сигнал завершения, останавливаем серверы")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("Ошибка при остановке HTTP сервера", "error", err)
	}

	if err := wsSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("Ошибка при остановке WebSocket сервера", "error", err)
	}

	log.Info("Серверы остановлены")
}
