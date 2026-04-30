package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler возвращает HTTP хендлер для /metrics
func Handler() http.Handler {
	return promhttp.Handler()
}