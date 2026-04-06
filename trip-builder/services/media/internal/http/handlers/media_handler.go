package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type MediaHandler struct {
	mediaService *service.MediaService
	log          *logger.Logger
}

func NewMediaHandler(mediaService *service.MediaService, log *logger.Logger) *MediaHandler {
	return &MediaHandler{
		mediaService: mediaService,
		log:          log,
	}
}

func (h *MediaHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   err,
		"details": details,
	})
}

func (h *MediaHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *MediaHandler) getUserIDFromContext(r *http.Request) *uuid.UUID {
	userIDStr, ok := r.Context().Value("user_id").(string)
	if !ok || userIDStr == "" {
		return nil
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.log.Debug("Неверный формат user_id в контексте", "user_id", userIDStr)
		return nil
	}

	return &userID
}

// Upload загружает файл
func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Info("Upload: загрузка файла")

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	// Парсим multipart form (макс 100MB)
	err := r.ParseMultipartForm(100 << 20)
	if err != nil {
		log.Debug("Upload: ошибка парсинга формы", "error", err)
		h.writeError(w, http.StatusBadRequest, "failed to parse form", err.Error())
		return
	}

	// Получаем файл
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Debug("Upload: файл не найден", "error", err)
		h.writeError(w, http.StatusBadRequest, "file is required", "")
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Error("Upload: ошибка чтения файла", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to read file", "")
		return
	}

	// Получаем параметры
	purpose := r.FormValue("purpose")
	if purpose == "" {
		purpose = "general"
	}

	var eventID, waypointID *uuid.UUID
	if eventIDStr := r.FormValue("event_id"); eventIDStr != "" {
		id, err := uuid.Parse(eventIDStr)
		if err == nil {
			eventID = &id
		}
	}
	if waypointIDStr := r.FormValue("waypoint_id"); waypointIDStr != "" {
		id, err := uuid.Parse(waypointIDStr)
		if err == nil {
			waypointID = &id
		}
	}

	log.Debug("Upload: параметры",
		"user_id", userID,
		"filename", header.Filename,
		"size", len(fileBytes),
		"purpose", purpose,
		"event_id", eventID,
		"waypoint_id", waypointID,
	)

	// Загружаем в сервис
	media, err := h.mediaService.Upload(r.Context(), *userID, fileBytes, header.Filename, purpose, eventID, waypointID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidFileType):
			h.writeError(w, http.StatusBadRequest, "invalid file type", "")
		case errors.Is(err, service.ErrFileTooLarge):
			h.writeError(w, http.StatusBadRequest, "file too large", "")
		default:
			log.Error("Upload: ошибка загрузки", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("Upload: файл загружен", "media_id", media.ID)

	// Возвращаем DTO без URL
	response := &models.MediaResponse{
		ID:        media.ID,
		MediaType: media.MediaType,
		FileSize:  media.FileSize,
		Width:     media.Width,
		Height:    media.Height,
		CreatedAt: media.CreatedAt,
	}

	h.writeJSON(w, http.StatusCreated, response)
}

// GetByID возвращает метаданные медиа
func (h *MediaHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	mediaIDStr := vars["id"]

	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		log.Debug("GetByID: неверный ID", "media_id", mediaIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid media id", "")
		return
	}

	media, err := h.mediaService.GetByID(r.Context(), mediaID)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			h.writeError(w, http.StatusNotFound, "media not found", "")
			return
		}
		log.Error("GetByID: ошибка получения", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	h.writeJSON(w, http.StatusOK, media)
}

// Delete удаляет медиа
func (h *MediaHandler) Delete(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	mediaIDStr := vars["id"]

	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		log.Debug("Delete: неверный ID", "media_id", mediaIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid media id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	err = h.mediaService.Delete(r.Context(), mediaID, *userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMediaNotFound):
			h.writeError(w, http.StatusNotFound, "media not found", "")
		case errors.Is(err, service.ErrNotAuthorized):
			h.writeError(w, http.StatusForbidden, "not authorized", "")
		default:
			log.Error("Delete: ошибка удаления", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("Delete: медиа удалено", "media_id", mediaID)
	h.writeJSON(w, http.StatusOK, map[string]string{"message": "media deleted successfully"})
}

// GetByEventID возвращает медиа поездки
func (h *MediaHandler) GetByEventID(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["eventId"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("GetByEventID: неверный ID события", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 || size > 10000 {
		size = 10000
	}

	mediaList, total, err := h.mediaService.GetByEventID(r.Context(), eventID, page, size)
	if err != nil {
		log.Error("GetByEventID: ошибка получения", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	totalPages := (int(total) + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}

	response := map[string]interface{}{
		"items":       mediaList,
		"total":       total,
		"page":        page,
		"size":        size,
		"total_pages": totalPages,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// GetByWaypointID возвращает медиа точки
func (h *MediaHandler) GetByWaypointID(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	waypointIDStr := vars["waypointId"]

	waypointID, err := uuid.Parse(waypointIDStr)
	if err != nil {
		log.Debug("GetByWaypointID: неверный ID точки", "waypoint_id", waypointIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid waypoint id", "")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 || size > 10000 {
		size = 10000
	}

	mediaList, total, err := h.mediaService.GetByWaypointID(r.Context(), waypointID, page, size)
	if err != nil {
		log.Error("GetByWaypointID: ошибка получения", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	totalPages := (int(total) + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}

	response := map[string]interface{}{
		"items":       mediaList,
		"total":       total,
		"page":        page,
		"size":        size,
		"total_pages": totalPages,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// ServeFile отдает файл клиенту
func (h *MediaHandler) ServeFile(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	mediaIDStr := vars["id"]

	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		log.Debug("ServeFile: неверный ID", "media_id", mediaIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid media id", "")
		return
	}

	// Получаем метаданные
	media, err := h.mediaService.GetByID(r.Context(), mediaID)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			h.writeError(w, http.StatusNotFound, "media not found", "")
			return
		}
		log.Error("ServeFile: ошибка получения метаданных", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	// Читаем файл из MinIO
	fileData, contentType, err := h.mediaService.GetFile(r.Context(), media.ObjectKey)
	if err != nil {
		log.Error("ServeFile: ошибка чтения файла", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	// Отдаем файл
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+media.Filename+"\"")
	w.Write(fileData)
}

// UploadBatch загружает несколько файлов
func (h *MediaHandler) UploadBatch(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Info("UploadBatch: множественная загрузка файлов")

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	// Парсим multipart form (макс 100MB * количество файлов)
	err := r.ParseMultipartForm(100 << 20)
	if err != nil {
		log.Debug("UploadBatch: ошибка парсинга формы", "error", err)
		h.writeError(w, http.StatusBadRequest, "failed to parse form", err.Error())
		return
	}

	// Получаем параметры
	purpose := r.FormValue("purpose")
	if purpose == "" {
		purpose = "general"
	}

	var eventID, waypointID *uuid.UUID
	if eventIDStr := r.FormValue("event_id"); eventIDStr != "" {
		id, err := uuid.Parse(eventIDStr)
		if err == nil {
			eventID = &id
		}
	}
	if waypointIDStr := r.FormValue("waypoint_id"); waypointIDStr != "" {
		id, err := uuid.Parse(waypointIDStr)
		if err == nil {
			waypointID = &id
		}
	}

	// Получаем все файлы
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		h.writeError(w, http.StatusBadRequest, "at least one file is required", "")
		return
	}

	results := make([]*models.MediaResponse, 0, len(files))
	errors := make([]map[string]string, 0)

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			errors = append(errors, map[string]string{
				"filename": fileHeader.Filename,
				"error":    "failed to open file",
			})
			continue
		}

		fileBytes, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			errors = append(errors, map[string]string{
				"filename": fileHeader.Filename,
				"error":    "failed to read file",
			})
			continue
		}

		media, err := h.mediaService.Upload(r.Context(), *userID, fileBytes, fileHeader.Filename, purpose, eventID, waypointID)
		if err != nil {
			errors = append(errors, map[string]string{
				"filename": fileHeader.Filename,
				"error":    err.Error(),
			})
			continue
		}

		results = append(results, &models.MediaResponse{
			ID:        media.ID,
			MediaType: media.MediaType,
			FileSize:  media.FileSize,
			Width:     media.Width,
			Height:    media.Height,
			CreatedAt: media.CreatedAt,
		})
	}

	response := map[string]interface{}{
		"uploaded": results,
		"failed":   errors,
	}

	h.writeJSON(w, http.StatusOK, response)
}
