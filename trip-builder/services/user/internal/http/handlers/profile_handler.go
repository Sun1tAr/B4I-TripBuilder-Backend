package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/client/event"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/client/media"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/service"
	commondto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/common"
	userdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/user"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ProfileHandler struct {
	profileService *service.ProfileService
	mediaClient    *media.Client
	eventClient    *event.Client
	log            *logger.Logger
}

func NewProfileHandler(
	profileService *service.ProfileService,
	mediaClient *media.Client,
	eventClient *event.Client,
	log *logger.Logger,
) *ProfileHandler {
	return &ProfileHandler{
		profileService: profileService,
		mediaClient:    mediaClient,
		eventClient:    eventClient,
		log:            log,
	}
}

func (h *ProfileHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(commondto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func (h *ProfileHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// profileToResponse преобразует модель профиля в DTO ответа
func profileToResponse(p *models.Profile) *userdto.ProfileResponse {
	resp := &userdto.ProfileResponse{
		UserID:               p.UserID,
		FirstName:            p.FirstName,
		LastName:             p.LastName,
		AvatarURL:            p.AvatarURL,
		Bio:                  p.Bio,
		Language:             p.Language,
		Timezone:             p.Timezone,
		NotificationsEnabled: p.NotificationsEnabled,
		CreatedAt:            p.CreatedAt,
		UpdatedAt:            p.UpdatedAt,
	}
	return resp
}

// profileToPublicResponse преобразует модель профиля в публичное DTO
func profileToPublicResponse(p *models.Profile) *userdto.PublicProfileResponse {
	return &userdto.PublicProfileResponse{
		UserID:    p.UserID,
		FirstName: p.FirstName,
		LastName:  p.LastName,
		AvatarURL: p.AvatarURL,
		Bio:       p.Bio,
	}
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

	h.writeJSON(w, http.StatusOK, profileToPublicResponse(profile))
}

// GetMyProfile получение своего профиля (с приватными полями)
func (h *ProfileHandler) GetMyProfile(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())

	log.Debug("PROFILE HANDLER: GetMyProfile started",
		"path", r.URL.Path,
		"method", r.Method,
	)

	userIDStr, ok := r.Context().Value("user_id").(string)
	if !ok {
		log.Error("PROFILE HANDLER: user_id not found in context")
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	log.Debug("PROFILE HANDLER: got user_id from context", "user_id", userIDStr)

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Error("PROFILE HANDLER: invalid user id format",
			"user_id", userIDStr,
			"error", err,
		)
		h.writeError(w, http.StatusBadRequest, "invalid user id", err.Error())
		return
	}

	log.Debug("PROFILE HANDLER: parsed user_id", "user_id", userID)

	profile, err := h.profileService.GetFullProfile(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProfileNotFound):
			log.Warn("PROFILE HANDLER: profile not found", "user_id", userID)
			h.writeError(w, http.StatusNotFound, "profile not found", "")
		default:
			log.Error("PROFILE HANDLER: error getting profile",
				"error", err,
				"user_id", userID,
			)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Debug("PROFILE HANDLER: profile found",
		"user_id", userID,
		"first_name", profile.FirstName,
		"last_name", profile.LastName,
	)

	h.writeJSON(w, http.StatusOK, profileToResponse(profile))
	log.Debug("PROFILE HANDLER: GetMyProfile finished")
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

	var req userdto.UpdateProfileRequest
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

	h.writeJSON(w, http.StatusOK, userdto.MessageResponse{Message: "profile updated successfully"})
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

	file, header, err := r.FormFile("avatar")
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "avatar file is required", "")
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to read file", "")
		return
	}

	avatarURL, err := h.mediaClient.UploadAvatar(r.Context(), userID.String(), fileBytes, header.Filename)
	if err != nil {
		h.log.Error("Ошибка загрузки аватара в Media Service", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to upload avatar", "")
		return
	}

	if err := h.profileService.UpdateAvatar(r.Context(), userID, avatarURL); err != nil {
		h.log.Error("Ошибка обновления аватара в БД", "error", err, "user_id", userID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"avatar_url": avatarURL})
}

// GetUserEvents получение списка событий пользователя
func (h *ProfileHandler) GetUserEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", "")
		return
	}

	allEvents, err := h.eventClient.GetUserEvents(r.Context(), userID.String())
	if err != nil {
		h.log.Error("Ошибка получения событий", "error", err, "user_id", userID)
		allEvents = []event.Event{}
	}

	// Фильтруем: оставляем только публичные события
	publicEvents := make([]event.Event, 0)
	for _, e := range allEvents {
		if e.IsPublic {
			publicEvents = append(publicEvents, e)
		}
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": userID,
		"events":  publicEvents,
		"total":   len(publicEvents),
	})
}
