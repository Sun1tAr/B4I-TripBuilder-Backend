package middleware

import (
	"net/http"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

func Logging(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			l := log.WithRequestID(r.Context())

			l.InfoContext(r.Context(), "Request started",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)

			l.Debug("Получен запрос: ", "uri", r.RequestURI)

			next.ServeHTTP(w, r)

			l.InfoContext(r.Context(), "Request completed",
				"duration", time.Since(start).String(),
			)
		})
	}
}
