package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/http/handlers/dto"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type FollowHandler struct {
	followService *service.FollowService
	log           *logger.Logger
}

func NewFollowHandler(followService *service.FollowService, log *logger.Logger) *FollowHandler {
	return &FollowHandler{
		followService: followService,
		log:           log,
	}
}

func (h *FollowHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(dto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func (h *FollowHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Follow подписаться на пользователя
func (h *FollowHandler) Follow(w http.ResponseWriter, r *http.Request) {
	followerIDStr, ok := r.Context().Value("user_id").(string)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	followerID, err := uuid.Parse(followerIDStr)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "invalid user id in token", "")
		return
	}

	vars := mux.Vars(r)
	followeeIDStr := vars["id"]

	followeeID, err := uuid.Parse(followeeIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid followee id", "")
		return
	}

	if followerID == followeeID {
		h.writeError(w, http.StatusBadRequest, "cannot follow yourself", "")
		return
	}

	if err := h.followService.Follow(r.Context(), followerID, followeeID); err != nil {
		switch {
		case errors.Is(err, service.ErrAlreadyFollowing):
			h.writeError(w, http.StatusConflict, "already following", "")
		case errors.Is(err, service.ErrProfileNotFound):
			h.writeError(w, http.StatusNotFound, "user not found", "")
		default:
			h.log.Error("Ошибка подписки", "error", err,
				"follower_id", followerID, "followee_id", followeeID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "followed successfully"})
}

// Unfollow отписаться от пользователя
func (h *FollowHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
	followerIDStr, ok := r.Context().Value("user_id").(string)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	followerID, err := uuid.Parse(followerIDStr)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "invalid user id in token", "")
		return
	}

	vars := mux.Vars(r)
	followeeIDStr := vars["id"]

	followeeID, err := uuid.Parse(followeeIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid followee id", "")
		return
	}

	if err := h.followService.Unfollow(r.Context(), followerID, followeeID); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFollowing):
			h.writeError(w, http.StatusNotFound, "not following", "")
		default:
			h.log.Error("Ошибка отписки", "error", err,
				"follower_id", followerID, "followee_id", followeeID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "unfollowed successfully"})
}

// GetFollowers получить список подписчиков
func (h *FollowHandler) GetFollowers(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", "")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	followers, total, err := h.followService.GetFollowers(r.Context(), userID, limit, offset)
	if err != nil {
		h.log.Error("Ошибка получения подписчиков", "error", err, "user_id", userID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":   userID,
		"followers": followers,
		"total":     total,
	})
}

// GetFollowing получить список подписок
func (h *FollowHandler) GetFollowing(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", "")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	following, total, err := h.followService.GetFollowing(r.Context(), userID, limit, offset)
	if err != nil {
		h.log.Error("Ошибка получения подписок", "error", err, "user_id", userID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":   userID,
		"following": following,
		"total":     total,
	})
}