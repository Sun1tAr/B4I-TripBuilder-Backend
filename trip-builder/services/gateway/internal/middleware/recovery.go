package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

func Recovery(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					l := log.WithRequestID(r.Context())
					l.ErrorContext(r.Context(), "Panic recovered",
						"panic", err,
						"stack", string(debug.Stack()),
					)
					handlers.WriteError(w, http.StatusInternalServerError, "internal server error", "")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}