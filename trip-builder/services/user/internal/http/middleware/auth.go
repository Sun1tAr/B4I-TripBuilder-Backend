package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/http/handlers/dto"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

func Auth(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Добавляем request_id в лог
			log = log.WithRequestID(r.Context())

			log.Debug("AUTH MIDDLEWARE: started",
				"path", r.URL.Path,
				"method", r.Method,
				"remote_addr", r.RemoteAddr,
				"headers", r.Header,
			)

			// Проверяем заголовок X-User-ID от gateway
			userID := r.Header.Get("X-User-ID")
			log.Debug("AUTH MIDDLEWARE: X-User-ID header", "value", userID)

			if userID == "" {
				log.Warn("AUTH MIDDLEWARE: missing X-User-ID header")
				writeError(w, http.StatusUnauthorized, "unauthorized", "missing user id")
				return
			}

			// Проверяем Authorization header на всякий случай
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				log.Debug("AUTH MIDDLEWARE: Authorization header present",
					"length", len(authHeader),
					"prefix", authHeader[:min(20, len(authHeader))],
				)
			}

			log.Debug("AUTH MIDDLEWARE: setting user_id in context", "user_id", userID)
			ctx := context.WithValue(r.Context(), "user_id", userID)

			next.ServeHTTP(w, r.WithContext(ctx))
			log.Debug("AUTH MIDDLEWARE:", "Next type:", next)
			log.Debug("AUTH MIDDLEWARE: finished")
		})
	}
}

func writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(dto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}