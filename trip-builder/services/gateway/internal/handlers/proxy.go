package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/config"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func WriteError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func Proxy(cfg *config.Config, serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var targetURL string
		switch serviceName {
		case "auth":
			targetURL = cfg.Services.AuthURL
		case "user":
			targetURL = cfg.Services.UserURL
		case "route":
			targetURL = cfg.Services.RouteURL
		case "event":
			targetURL = cfg.Services.EventURL
		case "task":
			targetURL = cfg.Services.TaskURL
		case "chat":
			targetURL = cfg.Services.ChatURL
		case "media":
			targetURL = cfg.Services.MediaURL
		case "search":
			targetURL = cfg.Services.SearchURL
		case "notification":
			targetURL = cfg.Services.NotificationURL
		default:
			WriteError(w, http.StatusInternalServerError, "unknown service", serviceName)
			return
		}

		target, err := url.Parse(targetURL)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "invalid target url", targetURL)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		r.Header.Set("X-Forwarded-Host", r.Host)
		r.Header.Set("X-Forwarded-Proto", "http")
		if r.TLS != nil {
			r.Header.Set("X-Forwarded-Proto", "https")
		}
		r.Header.Set("X-Forwarded-For", r.RemoteAddr)

		proxy.ServeHTTP(w, r)
	}
}

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "UP"})
}

func Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "READY"})
}