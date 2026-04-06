package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/service"
	commondto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/common"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type SearchHandler struct {
	searchService *service.SearchService
	log           *logger.Logger
}

func NewSearchHandler(searchService *service.SearchService, log *logger.Logger) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
		log:           log,
	}
}

func (h *SearchHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(commondto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func (h *SearchHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *SearchHandler) getUserIDFromContext(r *http.Request) string {
	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		return ""
	}
	return userID
}

// SearchRoutes поиск маршрутов
func (h *SearchHandler) SearchRoutes(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("SearchRoutes: запрос")

	query := r.URL.Query().Get("q")
	country := r.URL.Query().Get("country")
	tagsParam := r.URL.Query().Get("tags")
	sort := r.URL.Query().Get("sort")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}

	var tags []string
	if tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}

	var durationFrom, durationTo *int
	if df := r.URL.Query().Get("duration_from"); df != "" {
		if val, err := strconv.Atoi(df); err == nil && val > 0 {
			durationFrom = &val
		}
	}
	if dt := r.URL.Query().Get("duration_to"); dt != "" {
		if val, err := strconv.Atoi(dt); err == nil && val > 0 {
			durationTo = &val
		}
	}

	userID := h.getUserIDFromContext(r) // может быть пустым

	req := &models.RouteSearchRequest{
		Query:        query,
		Country:      country,
		Tags:         tags,
		DurationFrom: durationFrom,
		DurationTo:   durationTo,
		Sort:         sort,
		Page:         page,
		Size:         size,
		UserID:       userID, // добавляем поле
	}

	resp, err := h.searchService.SearchRoutes(r.Context(), req)
	if err != nil {
		log.Error("Ошибка поиска маршрутов", "error", err)
		h.writeError(w, http.StatusInternalServerError, "search failed", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// SearchEvents поиск публичных поездок
func (h *SearchHandler) SearchEvents(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("SearchEvents: запрос")

	query := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}
	if status == "" {
		status = "completed"
	}

	req := &models.EventSearchRequest{
		Query:  query,
		Status: status,
		Page:   page,
		Size:   size,
	}

	resp, err := h.searchService.SearchEvents(r.Context(), req)
	if err != nil {
		log.Error("Ошибка поиска событий", "error", err)
		h.writeError(w, http.StatusInternalServerError, "search failed", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// SearchUsers поиск пользователей
func (h *SearchHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("SearchUsers: запрос")

	query := r.URL.Query().Get("q")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}

	req := &models.UserSearchRequest{
		Query: query,
		Page:  page,
		Size:  size,
	}

	resp, err := h.searchService.SearchUsers(r.Context(), req)
	if err != nil {
		log.Error("Ошибка поиска пользователей", "error", err)
		h.writeError(w, http.StatusInternalServerError, "search failed", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// Suggest автодополнение
func (h *SearchHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("Suggest: запрос")

	query := r.URL.Query().Get("q")
	suggestType := r.URL.Query().Get("type")
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	if query == "" {
		h.writeJSON(w, http.StatusOK, &models.SuggestResponse{Suggestions: []models.Suggestion{}})
		return
	}
	if size < 1 || size > 20 {
		size = 10
	}

	resp, err := h.searchService.Suggest(r.Context(), query, suggestType, size)
	if err != nil {
		log.Error("Ошибка автодополнения", "error", err)
		h.writeError(w, http.StatusInternalServerError, "suggest failed", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// SyncFullReindex принудительная переиндексация (защищённая ручка)
func (h *SearchHandler) SyncFullReindex(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())

	// Проверяем авторизацию (только для внутреннего использования)
	userID := h.getUserIDFromContext(r)
	if userID == "" {
		log.Warn("SyncFullReindex: неавторизованный доступ")
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	log.Info("Запуск принудительной переиндексации", "user_id", userID)

	if err := h.searchService.FullReindex(r.Context()); err != nil {
		log.Error("Ошибка принудительной переиндексации", "error", err)
		h.writeError(w, http.StatusInternalServerError, "reindex failed", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "full reindex completed successfully"})
}
