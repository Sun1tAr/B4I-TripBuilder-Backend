package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/service"
	chatdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/chat"
	commondto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/common"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ChatHandler struct {
	chatService *service.ChatService
	log         *logger.Logger
}

func NewChatHandler(chatService *service.ChatService, log *logger.Logger) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		log:         log,
	}
}

func (h *ChatHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(commondto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func (h *ChatHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *ChatHandler) getUserIDFromContext(r *http.Request) *uuid.UUID {
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

func messageToResponse(msg *models.Message) *chatdto.MessageResponse {
	resp := &chatdto.MessageResponse{
		ID:        msg.ID,
		EventID:   msg.EventID,
		UserID:    msg.UserID,
		Text:      msg.Text,
		Type:      string(msg.Type),
		CreatedAt: msg.CreatedAt,
		UpdatedAt: msg.UpdatedAt,
	}
	if msg.Metadata != nil {
		resp.Metadata = msg.Metadata
	}
	return resp
}

// GetMessages возвращает историю сообщений
func (h *ChatHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("GetMessages: неверный ID события", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 || size > 100 {
		size = 50
	}

	log.Debug("GetMessages: запрос", "event_id", eventID, "user_id", userID, "page", page, "size", size)

	messages, total, err := h.chatService.GetMessages(r.Context(), eventID, page, size, *userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotParticipant):
			h.writeError(w, http.StatusForbidden, "you are not a participant of this event", "")
		default:
			log.Error("Ошибка получения сообщений", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	items := make([]chatdto.MessageResponse, len(messages))
	for i, msg := range messages {
		items[i] = *messageToResponse(&msg)
	}

	totalPages := (int(total) + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}

	response := chatdto.ListMessagesResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		Size:       size,
		TotalPages: totalPages,
	}

	log.Info("GetMessages: сообщения получены", "event_id", eventID, "count", len(items), "total", total)
	h.writeJSON(w, http.StatusOK, response)
}

// SendMessage отправляет сообщение (REST fallback)
func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("SendMessage: неверный ID события", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req chatdto.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("SendMessage: ошибка декодирования", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if req.Text == "" {
		h.writeError(w, http.StatusBadRequest, "text is required", "")
		return
	}

	log.Debug("SendMessage: отправка", "event_id", eventID, "user_id", userID, "text_len", len(req.Text))

	msg, err := h.chatService.SaveMessage(r.Context(), eventID, *userID, req.Text)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotParticipant):
			h.writeError(w, http.StatusForbidden, "you are not a participant of this event", "")
		default:
			log.Error("Ошибка отправки сообщения", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	// Публикуем в Redis для WebSocket
	go h.chatService.PublishMessage(r.Context(), eventID, msg)

	log.Info("SendMessage: сообщение отправлено", "message_id", msg.ID, "event_id", eventID)
	h.writeJSON(w, http.StatusCreated, messageToResponse(msg))
}

// UpdateMessage обновляет сообщение
func (h *ChatHandler) UpdateMessage(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	messageIDStr := vars["id"]

	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		log.Debug("UpdateMessage: неверный ID сообщения", "message_id", messageIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid message id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req chatdto.UpdateMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("UpdateMessage: ошибка декодирования", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if req.Text == "" {
		h.writeError(w, http.StatusBadRequest, "text is required", "")
		return
	}

	log.Debug("UpdateMessage: обновление", "message_id", messageID, "user_id", userID)

	msg, err := h.chatService.UpdateMessage(r.Context(), messageID, *userID, req.Text)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMessageNotFound):
			h.writeError(w, http.StatusNotFound, "message not found", "")
		case errors.Is(err, service.ErrNotAuthorized):
			h.writeError(w, http.StatusForbidden, "not authorized to edit this message", "")
		default:
			log.Error("Ошибка обновления сообщения", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("UpdateMessage: сообщение обновлено", "message_id", messageID)
	h.writeJSON(w, http.StatusOK, messageToResponse(msg))
}

// DeleteMessage удаляет сообщение
func (h *ChatHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	messageIDStr := vars["id"]

	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		log.Debug("DeleteMessage: неверный ID сообщения", "message_id", messageIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid message id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	log.Debug("DeleteMessage: удаление", "message_id", messageID, "user_id", userID)

	err = h.chatService.DeleteMessage(r.Context(), messageID, *userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMessageNotFound):
			h.writeError(w, http.StatusNotFound, "message not found", "")
		case errors.Is(err, service.ErrNotAuthorized):
			h.writeError(w, http.StatusForbidden, "not authorized to delete this message", "")
		default:
			log.Error("Ошибка удаления сообщения", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("DeleteMessage: сообщение удалено", "message_id", messageID)
	h.writeJSON(w, http.StatusOK, map[string]string{"message": "message deleted successfully"})
}

// GetUnreadCount возвращает количество непрочитанных сообщений
func (h *ChatHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("GetUnreadCount: неверный ID события", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	log.Debug("GetUnreadCount: запрос", "event_id", eventID, "user_id", userID)

	count, err := h.chatService.GetUnreadCount(r.Context(), eventID, *userID)
	if err != nil {
		log.Error("Ошибка получения количества непрочитанных", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	h.writeJSON(w, http.StatusOK, chatdto.UnreadCountResponse{Count: count})
}
