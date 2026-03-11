package handlers

import (
	"net/http"
)

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"UP"}`))
}

func Ready(w http.ResponseWriter, r *http.Request) {
	// Здесь можно добавить проверки зависимостей (БД, Kafka и т.д.)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"READY"}`))
}