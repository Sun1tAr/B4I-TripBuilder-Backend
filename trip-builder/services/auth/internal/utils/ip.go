package utils

import (
	"net/http"
	"strings"
)

// GetRealIP извлекает реальный IP клиента из заголовков X-Forwarded-For или X-Real-IP
func GetRealIP(r *http.Request) string {
	// Проверяем X-Forwarded-For (цепочка прокси)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Берём первый IP в списке (реальный клиент)
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}

	// Проверяем X-Real-IP (часто используется в nginx)
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Если ничего нет, возвращаем RemoteAddr (но это будет IP gateway)
	return r.RemoteAddr
}