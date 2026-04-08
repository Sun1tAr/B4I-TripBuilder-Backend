package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	commondto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/common"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

// AuthOptional – условная авторизация
func AuthOptional(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := log.WithRequestID(r.Context())

			userID := r.Header.Get("X-User-ID")
			if userID != "" {
				log.Debug("AuthOptional: пользователь идентифицирован", "user_id", userID)
				ctx := context.WithValue(r.Context(), "user_id", userID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			} else {
				log.Debug("AuthOptional: анонимный доступ", "headers", r.Header)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AuthRequired – обязательная авторизация (для /users и /sync)
func AuthRequired(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := log.WithRequestID(r.Context())

			userID := r.Header.Get("X-User-ID")
			if userID == "" {
				log.Warn("AuthRequired: missing X-User-ID header")
				writeError(w, http.StatusUnauthorized, "authorization required", "")
				return
			}

			log.Debug("AuthRequired: user authenticated", "user_id", userID)
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
