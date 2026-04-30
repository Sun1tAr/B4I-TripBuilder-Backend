package middleware

import (
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

func Logging(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			l := log.WithRequestID(r.Context())

			cleanPath := path.Clean(r.URL.Path)
			if strings.HasSuffix(r.URL.Path, "/") && !strings.HasSuffix(cleanPath, "/") {
				cleanPath += "/"
			}

			// Системные запросы логируем как DEBUG
			if isSystemPath(cleanPath) {
				l.DebugContext(r.Context(), "system request",
					"method", r.Method,
					"path", cleanPath,
				)
				next.ServeHTTP(w, r)
				return
			}

			l.InfoContext(r.Context(), "request started",
				"method", r.Method,
				"path", cleanPath,
				"remote_addr", r.RemoteAddr,
			)

			next.ServeHTTP(w, r)

			l.InfoContext(r.Context(), "request completed",
				"duration", time.Since(start).String(),
			)
		})
	}
}

// isSystemPath возвращает true для системных эндпоинтов
func isSystemPath(path string) bool {
	systemPaths := []string{"/metrics", "/health", "/ready"}
	for _, sp := range systemPaths {
		if path == sp || strings.HasPrefix(path, sp) {
			return true
		}
	}
	return false
}