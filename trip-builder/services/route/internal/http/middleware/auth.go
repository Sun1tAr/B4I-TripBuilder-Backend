package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	commondto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/common"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

func Auth(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := log.WithRequestID(r.Context())

			log.Debug("AUTH MIDDLEWARE: started",
				"path", r.URL.Path,
				"method", r.Method,
			)

			// Проверяем заголовок X-User-ID от gateway
			userID := r.Header.Get("X-User-ID")
			if userID == "" {
				log.Warn("AUTH MIDDLEWARE: missing X-User-ID header")
				writeError(w, http.StatusUnauthorized, "unauthorized", "missing user id")
				return
			}

			log.Debug("AUTH MIDDLEWARE: setting user_id in context", "user_id", userID)
			ctx := context.WithValue(r.Context(), "user_id", userID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(commondto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}
