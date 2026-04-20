package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/service"
	commondto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/common"
	notificationdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/notification"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type NotificationHandler struct {
	notificationService *service.NotificationService
	log                 *logger.Logger
}

func NewNotificationHandler(notificationService *service.NotificationService, log *logger.Logger) *NotificationHandler {
	log.Debug("NotificationHandler: инициализация")
	return &NotificationHandler{
		notificationService: notificationService,
		log:                 log,
	}
}

// writeError отправляет ошибку клиенту
func (h *NotificationHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	h.log.Debug("NotificationHandler.writeError",
		"status", status,
		"error", err,
		"details", details,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(commondto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

// writeJSON отправляет успешный JSON-ответ
func (h *NotificationHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	h.log.Debug("NotificationHandler.writeJSON",
		"status", status,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			h.log.Error("NotificationHandler.writeJSON: ошибка кодирования JSON",
				"error", err,
			)
		}
	}
}

// getUserIDFromContext извлекает user_id из контекста запроса
func (h *NotificationHandler) getUserIDFromContext(r *http.Request) (*uuid.UUID, error) {
	userIDStr, ok := r.Context().Value("user_id").(string)
	if !ok || userIDStr == "" {
		h.log.Debug("getUserIDFromContext: user_id не найден в контексте")
		return nil, nil
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.log.Debug("getUserIDFromContext: неверный формат user_id",
			"user_id_str", userIDStr,
			"error", err,
		)
		return nil, err
	}

	h.log.Debug("getUserIDFromContext: user_id извлечен",
		"user_id", userID,
	)
	return &userID, nil
}

// toNotificationResponse маппит внутреннюю модель в DTO
func toNotificationResponse(n *models.Notification) *notificationdto.NotificationResponse {
	if n == nil {
		return nil
	}

	// Конвертируем models.JSONMap в map[string]interface{}
	data := make(map[string]interface{})
	for k, v := range n.Data {
		data[k] = v
	}

	return &notificationdto.NotificationResponse{
		ID:        n.ID,
		Type:      n.Type,
		Title:     n.Title,
		Body:      n.Body,
		Data:      data,
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt,
	}
}

// toNotificationResponseList маппит список моделей в список DTO
func toNotificationResponseList(notifications []*models.Notification) []notificationdto.NotificationResponse {
	if notifications == nil {
		return []notificationdto.NotificationResponse{}
	}

	result := make([]notificationdto.NotificationResponse, 0, len(notifications))
	for _, n := range notifications {
		if resp := toNotificationResponse(n); resp != nil {
			result = append(result, *resp)
		}
	}
	return result
}

// maskToken маскирует токен для ответа
func maskToken(token string) string {
	if len(token) <= 10 {
		return "***"
	}
	return token[:5] + "..." + token[len(token)-5:]
}

// min возвращает минимум из двух чисел
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetNotifications возвращает список уведомлений пользователя
// GET /api/v1/notifications?page=1&size=20
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("GetNotifications: начало запроса",
		"path", r.URL.Path,
		"query", r.URL.RawQuery,
	)

	userID, err := h.getUserIDFromContext(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", err.Error())
		return
	}
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user_id not found")
		return
	}

	// Парсим параметры пагинации
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	size := 20
	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 && s <= 100 {
			size = s
		}
	}

	log.Debug("GetNotifications: параметры запроса",
		"user_id", userID,
		"page", page,
		"size", size,
	)

	notifications, total, err := h.notificationService.GetNotifications(r.Context(), *userID, page, size)
	if err != nil {
		log.Error("GetNotifications: ошибка получения уведомлений",
			"error", err,
			"user_id", userID,
		)
		h.writeError(w, http.StatusInternalServerError, "internal server error", err.Error())
		return
	}

	// Получаем количество непрочитанных
	unreadCount, err := h.notificationService.GetUnreadCount(r.Context(), *userID)
	if err != nil {
		log.Error("GetNotifications: ошибка получения непрочитанных",
			"error", err,
			"user_id", userID,
		)
		unreadCount = 0
	}

	totalPages := (int(total) + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}

	response := notificationdto.ListNotificationsResponse{
		Items:       toNotificationResponseList(notifications),
		Total:       total,
		Page:        page,
		Size:        size,
		TotalPages:  totalPages,
		UnreadCount: unreadCount,
	}

	log.Info("GetNotifications: уведомления получены",
		"user_id", userID,
		"count", len(notifications),
		"total", total,
		"unread", unreadCount,
	)

	h.writeJSON(w, http.StatusOK, response)
}

// GetUnreadCount возвращает количество непрочитанных уведомлений
// GET /api/v1/notifications/unread
func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("GetUnreadCount: начало запроса",
		"path", r.URL.Path,
	)

	userID, err := h.getUserIDFromContext(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", err.Error())
		return
	}
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user_id not found")
		return
	}

	log.Debug("GetUnreadCount: параметры",
		"user_id", userID,
	)

	count, err := h.notificationService.GetUnreadCount(r.Context(), *userID)
	if err != nil {
		log.Error("GetUnreadCount: ошибка получения количества",
			"error", err,
			"user_id", userID,
		)
		h.writeError(w, http.StatusInternalServerError, "internal server error", err.Error())
		return
	}

	log.Debug("GetUnreadCount: количество получено",
		"user_id", userID,
		"count", count,
	)

	h.writeJSON(w, http.StatusOK, notificationdto.UnreadCountResponse{
		Count: count,
	})
}

// MarkAsRead отмечает уведомление как прочитанное
// PUT /api/v1/notifications/{id}/read
func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())

	vars := mux.Vars(r)
	idStr := vars["id"]

	log.Debug("MarkAsRead: начало запроса",
		"path", r.URL.Path,
		"notification_id", idStr,
	)

	userID, err := h.getUserIDFromContext(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", err.Error())
		return
	}
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user_id not found")
		return
	}

	notificationID, err := uuid.Parse(idStr)
	if err != nil {
		log.Debug("MarkAsRead: неверный ID уведомления",
			"id_str", idStr,
			"error", err,
		)
		h.writeError(w, http.StatusBadRequest, "invalid notification id", err.Error())
		return
	}

	log.Debug("MarkAsRead: параметры",
		"user_id", userID,
		"notification_id", notificationID,
	)

	if err := h.notificationService.MarkAsRead(r.Context(), notificationID, *userID); err != nil {
		log.Error("MarkAsRead: ошибка отметки прочитанным",
			"error", err,
			"user_id", userID,
			"notification_id", notificationID,
		)
		h.writeError(w, http.StatusInternalServerError, "internal server error", err.Error())
		return
	}

	log.Info("MarkAsRead: уведомление отмечено прочитанным",
		"user_id", userID,
		"notification_id", notificationID,
	)

	h.writeJSON(w, http.StatusOK, notificationdto.MessageResponse{
		Message: "notification marked as read",
	})
}

// MarkAllAsRead отмечает все уведомления пользователя как прочитанные
// PUT /api/v1/notifications/read-all
func (h *NotificationHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("MarkAllAsRead: начало запроса",
		"path", r.URL.Path,
	)

	userID, err := h.getUserIDFromContext(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", err.Error())
		return
	}
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user_id not found")
		return
	}

	log.Debug("MarkAllAsRead: параметры",
		"user_id", userID,
	)

	count, err := h.notificationService.MarkAllAsRead(r.Context(), *userID)
	if err != nil {
		log.Error("MarkAllAsRead: ошибка отметки всех прочитанными",
			"error", err,
			"user_id", userID,
		)
		h.writeError(w, http.StatusInternalServerError, "internal server error", err.Error())
		return
	}

	log.Info("MarkAllAsRead: все уведомления отмечены прочитанными",
		"user_id", userID,
		"count", count,
	)

	h.writeJSON(w, http.StatusOK, notificationdto.MarkAllAsReadResponse{
		Message: "all notifications marked as read",
		Count:   count,
	})
}

// RegisterDevice регистрирует устройство для push-уведомлений
// POST /api/v1/notifications/device
func (h *NotificationHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("RegisterDevice: начало запроса",
		"path", r.URL.Path,
	)

	userID, err := h.getUserIDFromContext(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", err.Error())
		return
	}
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user_id not found")
		return
	}

	var req notificationdto.RegisterDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("RegisterDevice: ошибка декодирования запроса",
			"error", err,
		)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Валидация
	if req.Token == "" {
		h.writeError(w, http.StatusBadRequest, "token is required", "")
		return
	}
	if req.Platform == "" {
		req.Platform = models.PlatformWeb
	}
	if req.Platform != models.PlatformWeb && req.Platform != models.PlatformIOS && req.Platform != models.PlatformAndroid {
		h.writeError(w, http.StatusBadRequest, "invalid platform, must be web, ios or android", "")
		return
	}

	log.Debug("RegisterDevice: данные запроса",
		"user_id", userID,
		"platform", req.Platform,
		"token_prefix", req.Token[:min(10, len(req.Token))]+"...",
	)

	if err := h.notificationService.RegisterDevice(r.Context(), *userID, req.Token, req.Platform); err != nil {
		log.Error("RegisterDevice: ошибка регистрации устройства",
			"error", err,
			"user_id", userID,
			"platform", req.Platform,
		)
		h.writeError(w, http.StatusInternalServerError, "internal server error", err.Error())
		return
	}

	log.Info("RegisterDevice: устройство зарегистрировано",
		"user_id", userID,
		"platform", req.Platform,
	)

	h.writeJSON(w, http.StatusCreated, notificationdto.RegisterDeviceResponse{
		UserID:   *userID,
		Token:    maskToken(req.Token),
		Platform: req.Platform,
		Message:  "device registered successfully",
	})
}

// UnregisterDevice удаляет устройство
// DELETE /api/v1/notifications/device/{token}
func (h *NotificationHandler) UnregisterDevice(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())

	vars := mux.Vars(r)
	token := vars["token"]

	log.Debug("UnregisterDevice: начало запроса",
		"path", r.URL.Path,
	)

	userID, err := h.getUserIDFromContext(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", err.Error())
		return
	}
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "user_id not found")
		return
	}

	if token == "" {
		h.writeError(w, http.StatusBadRequest, "token is required", "")
		return
	}

	log.Debug("UnregisterDevice: параметры",
		"user_id", userID,
		"token_prefix", token[:min(10, len(token))]+"...",
	)

	if err := h.notificationService.UnregisterDevice(r.Context(), *userID, token); err != nil {
		log.Error("UnregisterDevice: ошибка удаления устройства",
			"error", err,
			"user_id", userID,
		)
		h.writeError(w, http.StatusInternalServerError, "internal server error", err.Error())
		return
	}

	log.Info("UnregisterDevice: устройство удалено",
		"user_id", userID,
	)

	h.writeJSON(w, http.StatusOK, notificationdto.MessageResponse{
		Message: "device unregistered successfully",
	})
}
