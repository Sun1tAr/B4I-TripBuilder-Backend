package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/gateway/internal/handlers"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				handlers.WriteError(w, http.StatusUnauthorized, "missing authorization header", "")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				handlers.WriteError(w, http.StatusUnauthorized, "invalid authorization header format", "")
				return
			}

			tokenString := parts[1]

			// Парсим и валидируем токен
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				handlers.WriteError(w, http.StatusUnauthorized, "invalid token", err.Error())
				return
			}

			// Извлекаем user_id из claims
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				if userID, ok := claims["user_id"].(string); ok {
					ctx := context.WithValue(r.Context(), UserIDKey, userID)
					r.Header.Set("X-User-ID", userID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			handlers.WriteError(w, http.StatusUnauthorized, "invalid token claims", "")
		})
	}
}