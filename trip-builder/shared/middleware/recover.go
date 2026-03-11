package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

func Recover(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.WithRequestID(r.Context()).ErrorContext(r.Context(), "panic recovered",
						"panic", err,
						"stack", string(debug.Stack()),
					)
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}