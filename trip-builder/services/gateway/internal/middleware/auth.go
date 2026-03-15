package middleware

import (
	"context"
	"net/http"
	"strings"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/handlers"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func Auth(jwtSecret string, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Логируем входящий запрос
			log.Debug("Auth middleware: processing request",
				"path", r.URL.Path,
				"method", r.Method,
				"headers", r.Header,
			)

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Warn("Auth middleware: missing authorization header")
				handlers.WriteError(w, http.StatusUnauthorized, "missing authorization header", "")
				return
			}

			log.Debug("Auth middleware: auth header", "header", authHeader)

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				log.Warn("Auth middleware: invalid authorization header format", "parts", parts)
				handlers.WriteError(w, http.StatusUnauthorized, "invalid authorization header format", "")
				return
			}

			tokenString := parts[1]
			log.Debug("Auth middleware: token received", "token_length", len(tokenString))

			// Парсим и валидируем токен
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					log.Warn("Auth middleware: unexpected signing method", "method", token.Header["alg"])
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})

			if err != nil {
				log.Warn("Auth middleware: token parse error", "error", err)
				handlers.WriteError(w, http.StatusUnauthorized, "invalid token", err.Error())
				return
			}

			if !token.Valid {
				log.Warn("Auth middleware: token invalid")
				handlers.WriteError(w, http.StatusUnauthorized, "invalid token", "")
				return
			}

			// Извлекаем claims
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				log.Warn("Auth middleware: invalid claims type")
				handlers.WriteError(w, http.StatusUnauthorized, "invalid token claims", "")
				return
			}

			log.Debug("Auth middleware: token claims", "claims", claims)

			// Извлекаем user_id
			userIDRaw, exists := claims["user_id"]
			if !exists {
				log.Warn("Auth middleware: user_id not found in claims")
				handlers.WriteError(w, http.StatusUnauthorized, "invalid token claims", "user_id missing")
				return
			}

			log.Debug("Auth middleware: raw user_id", "type", typeof(userIDRaw), "value", userIDRaw)

			// Преобразуем в строку
			var userID string
			switch v := userIDRaw.(type) {
			case string:
				userID = v
			case float64:
				// Если вдруг пришло число (JWT иногда парсит числа как float64)
				userID = fmt.Sprintf("%.0f", v)
				log.Warn("Auth middleware: user_id was number, converted to string", "original", v, "converted", userID)
			default:
				log.Error("Auth middleware: user_id has unexpected type", "type", typeof(v), "value", v)
				handlers.WriteError(w, http.StatusUnauthorized, "invalid token claims", "user_id invalid type")
				return
			}

			log.Debug("Auth middleware: setting X-User-ID header", "user_id", userID)

			// Устанавливаем заголовок и контекст
			r.Header.Set("X-User-ID", userID)
			ctx := context.WithValue(r.Context(), UserIDKey, userID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Вспомогательная функция для определения типа
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