package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	userdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/user"
	commondto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/common"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type StatsHandler struct {
	statsService *service.StatsService
	log          *logger.Logger
}

// statsToResponse преобразует результат сервиса в DTO ответа
func statsToResponse(s *service.StatsResult) *userdto.StatsResponse {
	return &userdto.StatsResponse{
		UserID:          s.UserID,
		RoutesCreated:   s.RoutesCreated,
		EventsDone:      s.EventsDone,
		CountriesVisited: s.CountriesVisited,
		PhotosUploaded:   s.PhotosUploaded,
		Followers:        s.Followers,
		Following:        s.Following,
	}
}

func NewStatsHandler(statsService *service.StatsService, log *logger.Logger) *StatsHandler {
	return &StatsHandler{
		statsService: statsService,
		log:          log,
	}
}

func (h *StatsHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(commondto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func (h *StatsHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *StatsHandler) GetUserStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", "")
		return
	}

	stats, err := h.statsService.GetUserStats(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrStatsNotFound):
			// Возвращаем пустую статистику, а не ошибку
			emptyStats := &service.StatsResult{UserID: userID}
			h.writeJSON(w, http.StatusOK, statsToResponse(emptyStats))
			return
		default:
			h.log.Error("Ошибка получения статистики", "error", err, "user_id", userID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
			return
		}
	}

	h.writeJSON(w, http.StatusOK, statsToResponse(stats))
}