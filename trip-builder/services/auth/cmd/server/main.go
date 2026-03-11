package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/http/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/middleware"
	"github.com/joho/godotenv"
)

func main() {
	// Загружаем .env файл (для разработки)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	cfg := config.MustLoad()

	logger := logger.New(logger.Options{
		Level:       cfg.LogLevel(),
		ServiceName: cfg.ServiceName,
		Format:      cfg.LogFormat,
	})

	logger.Info("запуск сервиса", "port", cfg.HTTPPort)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handlers.Health)
	mux.HandleFunc("GET /ready", handlers.Ready)

	handler := middleware.RequestID(mux)
	handler = middleware.Logging(logger)(handler)
	handler = middleware.Recover(logger)(handler)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("ошибка запуска сервера", "error", err)
			os.Exit(1)
		}
	}()

	logger.Info("сервер запущен", "port", cfg.HTTPPort)
	<-done
	logger.Info("получен сигнал завершения, останавливаемся")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("ошибка при graceful shutdown", "error", err)
	}

	logger.Info("сервер остановлен")
}