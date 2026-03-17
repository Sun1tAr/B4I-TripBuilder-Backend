package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	userdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/user"
	commondto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/common"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type AchievementHandler struct {
	achievementService *service.AchievementService
	log                *logger.Logger
}

func NewAchievementHandler(achievementService *service.AchievementService, log *logger.Logger) *AchievementHandler {
	return &AchievementHandler{
		achievementService: achievementService,
		log:                log,
	}
}

func (h *AchievementHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(commondto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func (h *AchievementHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// achievementToResponse преобразует модель достижения в DTO ответа
func achievementToResponse(a *models.Achievement) *userdto.AchievementResponse {
	return &userdto.AchievementResponse{
		ID:          a.ID,
		Type:        a.Type,
		Title:       a.Title,
		Description: a.Description,
		EarnedAt:    a.EarnedAt,
	}
}

func (h *AchievementHandler) GetUserAchievements(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", "")
		return
	}

	achievements, err := h.achievementService.GetUserAchievements(r.Context(), userID)
	if err != nil {
		h.log.Error("Ошибка получения достижений", "error", err, "user_id", userID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	var resp []*userdto.AchievementResponse
	for _, a := range achievements {
		resp = append(resp, achievementToResponse(a))
	}
	h.writeJSON(w, http.StatusOK, resp)
}