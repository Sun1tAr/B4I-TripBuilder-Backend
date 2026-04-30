package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/handlers"
	"golang.org/x/time/rate"
)

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	clients = make(map[string]*client)
	mu      sync.Mutex
)

func init() {
	// Запускаем горутину для очистки старых записей
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()
}

func RateLimit(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем IP (учитываем X-Forwarded-For)
			ip := r.RemoteAddr
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = forwarded
			}

			mu.Lock()
			defer mu.Unlock()

			// Создаём или получаем лимитер для IP
			c, exists := clients[ip]
			if !exists {
				limiter := rate.NewLimiter(rate.Limit(cfg.RateLimitRequests)/rate.Limit(cfg.RateLimitWindow.Seconds()), cfg.RateLimitRequests)
				clients[ip] = &client{limiter: limiter, lastSeen: time.Now()}
				c = clients[ip]
			} else {
				c.lastSeen = time.Now()
			}

			if !c.limiter.Allow() {
				handlers.WriteError(w, http.StatusTooManyRequests, "rate limit exceeded", "")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}