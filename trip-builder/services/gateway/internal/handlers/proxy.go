package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

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

// WebSocketProxy проксирует WebSocket соединение к целевому сервису
func WebSocketProxy(cfg *config.Config, serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.New(logger.Options{
			ServiceName: "gateway",
			Level:       logger.LevelInfo,
			Format:      "console",
		})

		var targetURL string
		switch serviceName {
		case "chat":
			targetURL = cfg.Services.ChatURL
		default:
			WriteError(w, http.StatusInternalServerError, "unknown service for websocket", serviceName)
			return
		}

		// Извлекаем токен из query-параметра
		token := r.URL.Query().Get("token")
		if token == "" {
			// Fallback: пробуем из заголовка (для совместимости)
			token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}

		if token == "" {
			log.Warn("WebSocketProxy: token not found in query or header")
			WriteError(w, http.StatusUnauthorized, "unauthorized", "missing token")
			return
		}

		// Валидируем токен через Auth Service
		userID, err := validateToken(token, cfg.JWTSecret)
		if err != nil {
			log.Warn("WebSocketProxy: invalid token", "error", err)
			WriteError(w, http.StatusUnauthorized, "invalid token", "")
			return
		}

		// Строим целевой URL для WebSocket
		target := strings.Replace(targetURL, "http://", "ws://", 1)
		target = strings.Replace(target, "https://", "wss://", 1)

		path := r.URL.Path
		wsURL := target + path
		if r.URL.RawQuery != "" {
			wsURL += "?" + r.URL.RawQuery
		}
		// Убираем token из query, чтобы не светился в логах
		wsURL = strings.Replace(wsURL, "token="+token, "", -1)
		wsURL = strings.TrimSuffix(wsURL, "?&")
		wsURL = strings.TrimSuffix(wsURL, "&?")

		log.Info("WebSocketProxy: проксирование", "from", r.URL.Path, "to", wsURL, "user_id", userID)

		dialer := websocket.Dialer{}
		headers := http.Header{}
		headers.Set("X-User-ID", userID)

		targetConn, _, err := dialer.Dial(wsURL, headers)
		if err != nil {
			log.Error("WebSocketProxy: ошибка подключения", "error", err, "url", wsURL)
			WriteError(w, http.StatusBadGateway, "failed to connect to upstream", err.Error())
			return
		}
		defer targetConn.Close()

		clientConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("WebSocketProxy: ошибка апгрейда", "error", err)
			return
		}
		defer clientConn.Close()

		// Проксируем сообщения
		errChan := make(chan error, 2)

		go func() {
			for {
				msgType, msg, err := clientConn.ReadMessage()
				if err != nil {
					errChan <- err
					return
				}
				if err := targetConn.WriteMessage(msgType, msg); err != nil {
					errChan <- err
					return
				}
			}
		}()

		go func() {
			for {
				msgType, msg, err := targetConn.ReadMessage()
				if err != nil {
					errChan <- err
					return
				}
				if err := clientConn.WriteMessage(msgType, msg); err != nil {
					errChan <- err
					return
				}
			}
		}()

		<-errChan
	}
}

// validateToken проверяет JWT и возвращает user_id
func validateToken(tokenString, jwtSecret string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", jwt.ErrInvalidKey
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", jwt.ErrInvalidKey
	}

	return userID, nil
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
