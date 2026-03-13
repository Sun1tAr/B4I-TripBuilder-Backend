package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/http/handlers/dto"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ProfileHandler struct {
	profileService *service.ProfileService
	log            *logger.Logger
}

func NewProfileHandler(profileService *service.ProfileService, log *logger.Logger) *ProfileHandler {
	return &ProfileHandler{
		profileService: profileService,
		log:            log,
	}
}

func (h *ProfileHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(dto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func (h *ProfileHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// GetPublicProfile получение публичного профиля
func (h *ProfileHandler) GetPublicProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", "")
		return
	}

	profile, err := h.profileService.GetPublicProfile(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProfileNotFound):
			h.writeError(w, http.StatusNotFound, "profile not found", "")
		default:
			h.log.Error("Ошибка получения профиля", "error", err, "user_id", userID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, dto.ProfileToPublicResponse(profile))
}

// GetMyProfile получение своего профиля (с приватными полями)
func (h *ProfileHandler) GetMyProfile(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := r.Context().Value("user_id").(string)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "invalid user id in token", "")
		return
	}

	profile, err := h.profileService.GetFullProfile(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProfileNotFound):
			h.writeError(w, http.StatusNotFound, "profile not found", "")
		default:
			h.log.Error("Ошибка получения профиля", "error", err, "user_id", userID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, dto.ProfileToResponse(profile))
}

// UpdateProfile обновление профиля
func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := r.Context().Value("user_id").(string)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "invalid user id in token", "")
		return
	}

	var req dto.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if err := h.profileService.UpdateProfile(r.Context(), userID, &req); err != nil {
		switch {
		case errors.Is(err, service.ErrProfileNotFound):
			h.writeError(w, http.StatusNotFound, "profile not found", "")
		default:
			h.log.Error("Ошибка обновления профиля", "error", err, "user_id", userID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "profile updated successfully"})
}

// UploadAvatar загрузка аватара
func (h *ProfileHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := r.Context().Value("user_id").(string)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "invalid user id in token", "")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.writeError(w, http.StatusBadRequest, "failed to parse form", err.Error())
		return
	}

	file, _, err := r.FormFile("avatar") // убрали переменную header
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "avatar file is required", "")
		return
	}
	defer file.Close()

	// Читаем файл для проверки
	_, err = io.ReadAll(file) // присваиваем ошибке, но не используем fileBytes
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to read file", "")
		return
	}

	// TODO: отправить файл в Media Service
	avatarURL := "https://example.com/avatars/" + userID.String() + ".jpg"

	if err := h.profileService.UpdateAvatar(r.Context(), userID, avatarURL); err != nil {
		h.log.Error("Ошибка обновления аватара", "error", err, "user_id", userID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"avatar_url": avatarURL})
}

// GetUserEvents получение списка событий пользователя (прокси в event-service)
func (h *ProfileHandler) GetUserEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", "")
		return
	}

	// TODO: сделать HTTP-вызов в event-service
	// limit, offset из query params
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	// Пока заглушка
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": userID,
		"events":  []interface{}{},
		"total":   0,
	})
}