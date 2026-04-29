package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/mux"
)

// responseWriter обертка для получения статус-кода
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// ChiMetricsMiddleware собирает HTTP-метрики для chi.Router
func ChiMetricsMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пропускаем метрики, health и WebSocket
			if r.URL.Path == "/metrics" || r.URL.Path == "/health" || r.URL.Path == "/ready" || strings.Contains(r.URL.Path, "/ws/") {
				next.ServeHTTP(w, r)
				return
			}

			metrics.HTTPRequestsInFlight.WithLabelValues(serviceName).Inc()
			defer metrics.HTTPRequestsInFlight.WithLabelValues(serviceName).Dec()

			start := time.Now()

			// Оборачиваем через chi middleware для получения статус-кода
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start).Seconds()
			statusCode := strconv.Itoa(wrapped.Status())

			path := normalizeChiPath(r)

			metrics.HTTPRequestsTotal.WithLabelValues(serviceName, r.Method, path, statusCode).Inc()
			metrics.HTTPRequestDuration.WithLabelValues(serviceName, r.Method, path).Observe(duration)
		})
	}
}

// MetricsMiddleware собирает HTTP-метрики с нормализацией пути (для gorilla/mux)
func MetricsMiddleware(serviceName string, router *mux.Router) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пропускаем метрики, health и WebSocket
			if r.URL.Path == "/metrics" || r.URL.Path == "/health" || r.URL.Path == "/ready" || strings.Contains(r.URL.Path, "/ws/") {
				next.ServeHTTP(w, r)
				return
			}

			metrics.HTTPRequestsInFlight.WithLabelValues(serviceName).Inc()
			defer metrics.HTTPRequestsInFlight.WithLabelValues(serviceName).Dec()

			start := time.Now()
			path := normalizeMuxPath(r, router)

			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start).Seconds()
			statusCode := strconv.Itoa(wrapped.statusCode)

			metrics.HTTPRequestsTotal.WithLabelValues(serviceName, r.Method, path, statusCode).Inc()
			metrics.HTTPRequestDuration.WithLabelValues(serviceName, r.Method, path).Observe(duration)
		})
	}
}

// normalizeMuxPath нормализует путь для gorilla/mux
func normalizeMuxPath(r *http.Request, router *mux.Router) string {
	if router != nil {
		var match mux.RouteMatch
		if router.Match(r, &match) {
			if tmpl, err := match.Route.GetPathTemplate(); err == nil {
				return tmpl
			}
		}
	}
	return manualNormalize(r.URL.Path)
}

// normalizeChiPath нормализует путь для chi
func normalizeChiPath(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if pattern := rctx.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return manualNormalize(r.URL.Path)
}

// manualNormalize ручная нормализация пути
func manualNormalize(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if len(part) == 36 && strings.Count(part, "-") == 4 {
			parts[i] = "{id}"
			continue
		}
		if len(part) > 50 && strings.Contains(part, ".") {
			parts[i] = "{token}"
			continue
		}
		if len(part) >= 32 && isHex(part) {
			parts[i] = "{hash}"
			continue
		}
	}
	return "/" + strings.Join(parts, "/")
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}