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

			// Если заголовок есть — пробуем валидировать
			userID, ok := extractAndValidateToken(r, jwtSecret, log)
			if !ok {
				log.Debug("AuthOptional: no token, proceeding as anonymous")
			} else {
				log.Debug("AuthOptional: token valid, setting user_id", "user_id", userID)
			}

			r.Header.Set("X-User-ID", userID)
			ctx := context.WithValue(r.Context(), UserIDKey, userID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractAndValidateToken извлекает и валидирует JWT, возвращает user_id и флаг успеха
// Поддерживает:
// 1. Authorization: Bearer <token> (стандартный заголовок)
// 2. ?token=<jwt> (query-параметр для WebSocket и медиа)
func extractAndValidateToken(r *http.Request, jwtSecret string, log *logger.Logger) (string, bool) {
	var tokenString string

	// Сначала пробуем из заголовка Authorization
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			tokenString = parts[1]
		}
	}

	// Если в заголовке нет, пробуем из query-параметра token
	if tokenString == "" {
		tokenString = r.URL.Query().Get("token")
		if tokenString != "" {
			log.Debug("extractAndValidateToken: token from query parameter")
		}
	}

	// Если токена нигде нет
	if tokenString == "" {
		return "", false
	}

	// Валидируем JWT
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		log.Debug("extractAndValidateToken: invalid token", "error", err)
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
