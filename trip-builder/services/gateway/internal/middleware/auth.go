package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// AuthRequired — для закрытых маршрутов (обязательная авторизация)
func AuthRequired(jwtSecret string, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Debug("AuthRequired: processing request",
				"path", r.URL.Path,
				"method", r.Method,
			)

			userID, ok := extractAndValidateToken(r, jwtSecret, log)
			if !ok {
				handlers.WriteError(w, http.StatusUnauthorized, "missing or invalid authorization token", "")
				return
			}

			log.Debug("AuthRequired: token valid", "user_id", userID)
			r.Header.Set("X-User-ID", userID)
			ctx := context.WithValue(r.Context(), UserIDKey, userID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthOptional — для условно-закрытых маршрутов
// - если токена нет — анонимный доступ
// - если токен есть и валидный — передаём user_id
// - если токен есть но невалидный — возвращаем 401
func AuthOptional(jwtSecret string, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Debug("AuthOptional: processing request",
				"path", r.URL.Path,
				"method", r.Method,
			)

			authHeader := r.Header.Get("Authorization")

			// Если заголовка нет — анонимный доступ
			if authHeader == "" {
				log.Debug("AuthOptional: no token, proceeding as anonymous")
				next.ServeHTTP(w, r)
				return
			}

			// Если заголовок есть — пробуем валидировать
			userID, ok := extractAndValidateToken(r, jwtSecret, log)
			if !ok {
				// Токен есть, но невалидный — возвращаем 401
				log.Warn("AuthOptional: invalid token, returning 401")
				handlers.WriteError(w, http.StatusUnauthorized, "invalid authorization token", "")
				return
			}

			log.Debug("AuthOptional: token valid, setting user_id", "user_id", userID)
			r.Header.Set("X-User-ID", userID)
			ctx := context.WithValue(r.Context(), UserIDKey, userID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractAndValidateToken извлекает и валидирует JWT, возвращает user_id и флаг успеха
func extractAndValidateToken(r *http.Request, jwtSecret string, log *logger.Logger) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", false
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", false
	}

	tokenString := parts[1]

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return "", false
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", false
	}

	userIDRaw, exists := claims["user_id"]
	if !exists {
		return "", false
	}

	var userID string
	switch v := userIDRaw.(type) {
	case string:
		userID = v
	case float64:
		userID = fmt.Sprintf("%.0f", v)
	default:
		return "", false
	}

	return userID, true
}

// Вспомогательная функция для определения типа (оставляем для совместимости)
func typeof(v interface{}) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "float64"
	case float32:
		return "float32"
	case int:
		return "int"
	case int64:
		return "int64"
	case bool:
		return "bool"
	case nil:
		return "nil"
	default:
		return "unknown"
	}
}
