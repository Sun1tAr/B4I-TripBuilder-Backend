package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/service"
	commondto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/common"
	eventdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/event"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type EventHandler struct {
	eventService *service.EventService
	log          *logger.Logger
}

func NewEventHandler(eventService *service.EventService, log *logger.Logger) *EventHandler {
	return &EventHandler{
		eventService: eventService,
		log:          log,
	}
}

func (h *EventHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(commondto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func (h *EventHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *EventHandler) getUserIDFromContext(r *http.Request) *uuid.UUID {
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

func (h *EventHandler) getUserEmailFromContext(r *http.Request) string {
	email, ok := r.Context().Value("user_email").(string)
	if !ok || email == "" {
		return ""
	}
	return email
}

// eventDetailToResponse преобразует детальную модель в DTO ответа
// eventDetailToResponse преобразует детальную модель в DTO ответа
func eventDetailToResponse(event *models.EventWithDetails) *eventdto.EventDetailResponse {
	resp := &eventdto.EventDetailResponse{
		EventResponse: eventdto.EventResponse{
			ID:          event.ID,
			RouteID:     event.RouteID,
			Title:       event.Title,
			Description: event.Description,
			OwnerID:     event.OwnerID,
			StartDate:   event.StartDate,
			EndDate:     event.EndDate,
			IsGroup:     event.IsGroup,
			Status:      event.Status,
			IsPublic:    event.IsPublic,
			CreatedAt:   event.CreatedAt,
			UpdatedAt:   event.UpdatedAt,
		},
		Participants: make([]eventdto.ParticipantResponse, len(event.Participants)),
		Waypoints:    make([]eventdto.WaypointResponse, len(event.Waypoints)),
	}

	for i, p := range event.Participants {
		resp.Participants[i] = eventdto.ParticipantResponse{
			UserID:          p.UserID,
			Role:            p.Role,
			ApprovedByOwner: p.ApprovedByOwner,
			ApprovedByUser:  p.ApprovedByUser,
			InvitedEmail:    p.InvitedEmail,
			JoinedAt:        p.JoinedAt,
		}
	}

	for i, w := range event.Waypoints {
		resp.Waypoints[i] = eventdto.WaypointResponse{
			ID:           w.ID,
			City:         w.City,
			Description:  w.Description,
			Latitude:     w.Latitude,
			Longitude:    w.Longitude,
			PlannedStart: w.PlannedStart,
			PlannedEnd:   w.PlannedEnd,
			ActualStart:  w.ActualStart,
			ActualEnd:    w.ActualEnd,
			Status:       w.Status,
			OrderIndex:   w.OrderIndex,
		}
	}

	return resp
}

// GetMyEvents возвращает поездки текущего пользователя (только подтверждённые)
func (h *EventHandler) GetMyEvents(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("GetMyEvents: запрос поездок пользователя")

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	pageStr := r.URL.Query().Get("page")
	sizeStr := r.URL.Query().Get("size")
	status := r.URL.Query().Get("status")

	log.Debug("GetMyEvents: параметры запроса",
		"page", pageStr,
		"size", sizeStr,
		"status", status,
		"user_id", userID,
	)

	var page, size int
	var err error

	// Если параметры пагинации не указаны — возвращаем все записи
	if pageStr == "" && sizeStr == "" {
		page = 1
		size = 10000
		log.Debug("GetMyEvents: пагинация не указана, возвращаем все записи")
	} else {
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}

		size, err = strconv.Atoi(sizeStr)
		if err != nil || size < 1 {
			size = 10
		}
		if size > 100 {
			size = 100
		}
	}

	events, total, err := h.eventService.GetMyEvents(r.Context(), *userID, status, page, size)
	if err != nil {
		log.Error("Ошибка получения поездок", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	// Получаем статистику участников для всех событий
	eventIDs := make([]uuid.UUID, len(events))
	for i, e := range events {
		eventIDs[i] = e.ID
	}
	statsMap, err := h.eventService.GetParticipantsStats(r.Context(), eventIDs)
	if err != nil {
		log.Error("Ошибка получения статистики участников", "error", err)
		// не фатально, продолжаем с пустой статистикой
		statsMap = make(map[uuid.UUID]eventdto.EventParticipantsStatsDTO)
	}

	// Преобразуем в DTO
	items := make([]eventdto.EventResponse, 0, len(events))
	for _, e := range events {
		stats := statsMap[e.ID]
		items = append(items, eventdto.EventResponse{
			ID:                e.ID,
			RouteID:           e.RouteID,
			Title:             e.Title,
			Description:       e.Description,
			OwnerID:           e.OwnerID,
			StartDate:         e.StartDate,
			EndDate:           e.EndDate,
			IsGroup:           e.IsGroup,
			Status:            e.Status,
			IsPublic:          e.IsPublic,
			CreatedAt:         e.CreatedAt,
			UpdatedAt:         e.UpdatedAt,
			TotalInvites:      stats.TotalInvites,
			TotalParticipants: stats.TotalParticipants,
		})
	}

	totalPages := (int(total) + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}

	response := eventdto.ListEventsResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		Size:       size,
		TotalPages: totalPages,
	}

	log.Info("GetMyEvents: поездки получены",
		"user_id", userID,
		"count", len(items),
		"total", total,
		"page", page,
	)

	h.writeJSON(w, http.StatusOK, response)
}

// GetPublicEvents возвращает публичные завершённые поездки
func (h *EventHandler) GetPublicEvents(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("GetPublicEvents: запрос публичных поездок")

	pageStr := r.URL.Query().Get("page")
	sizeStr := r.URL.Query().Get("size")

	log.Debug("GetPublicEvents: параметры запроса",
		"page", pageStr,
		"size", sizeStr,
	)

	var page, size int
	var err error

	// Если параметры пагинации не указаны — возвращаем все записи
	if pageStr == "" && sizeStr == "" {
		page = 1
		size = 10000
		log.Debug("GetPublicEvents: пагинация не указана, возвращаем все записи")
	} else {
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}

		size, err = strconv.Atoi(sizeStr)
		if err != nil || size < 1 {
			size = 10
		}
		if size > 100 {
			size = 100
		}
	}

	events, total, err := h.eventService.GetPublicEvents(r.Context(), page, size)
	if err != nil {
		log.Error("Ошибка получения публичных поездок", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	// Преобразуем в DTO
	items := make([]eventdto.EventResponse, 0, len(events))
	for _, e := range events {
		items = append(items, eventdto.EventResponse{
			ID:          e.ID,
			RouteID:     e.RouteID,
			Title:       e.Title,
			Description: e.Description,
			OwnerID:     e.OwnerID,
			StartDate:   e.StartDate,
			EndDate:     e.EndDate,
			IsGroup:     e.IsGroup,
			Status:      e.Status,
			IsPublic:    e.IsPublic,
			CreatedAt:   e.CreatedAt,
			UpdatedAt:   e.UpdatedAt,
		})
	}

	totalPages := (int(total) + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}

	response := eventdto.ListEventsResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		Size:       size,
		TotalPages: totalPages,
	}

	log.Info("GetPublicEvents: публичные поездки получены",
		"count", len(items),
		"total", total,
		"page", page,
	)

	h.writeJSON(w, http.StatusOK, response)
}

// GetEventByID возвращает детальную информацию о поездке
func (h *EventHandler) GetEventByID(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("GetEventByID: неверный ID", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	userID := h.getUserIDFromContext(r)

	log.Debug("GetEventByID: запрос", "event_id", eventID, "user_id", userID)

	event, err := h.eventService.GetEventByID(r.Context(), eventID, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEventNotFound):
			log.Debug("GetEventByID: событие не найдено", "event_id", eventID)
			h.writeError(w, http.StatusNotFound, "event not found", "")
		default:
			log.Error("Ошибка получения поездки", "error", err, "event_id", eventID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("GetEventByID: событие получено", "event_id", eventID)
	response := eventDetailToResponse(event)
	h.writeJSON(w, http.StatusOK, response)
}

// CreateEvent создаёт новую поездку
func (h *EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Info("CreateEvent: создание поездки") // INFO — бизнес-операция

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req eventdto.CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("CreateEvent: ошибка декодирования", "error", err) // DEBUG — проверка
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	log.Debug("CreateEvent: данные запроса",
		"route_id", req.RouteID,
		"title", req.Title,
		"start_date", req.StartDate,
		"is_group", req.IsGroup,
		"invited_emails_count", len(req.InvitedEmails),
	)

	createReq := &models.CreateEventRequest{
		RouteID:       req.RouteID,
		Title:         req.Title,
		Description:   req.Description,
		StartDate:     req.StartDate,
		IsGroup:       req.IsGroup,
		InvitedEmails: req.InvitedEmails,
	}

	event, err := h.eventService.CreateEvent(r.Context(), *userID, createReq)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidStartDate):
			log.Debug("CreateEvent: дата начала в прошлом", "start_date", req.StartDate)
			h.writeError(w, http.StatusBadRequest, "start date cannot be in the past", "")
		default:
			log.Error("Ошибка создания поездки", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("CreateEvent: поездка создана", "event_id", event.ID, "user_id", userID)
	response := eventDetailToResponse(event)
	h.writeJSON(w, http.StatusCreated, response)
}

// UpdateEvent обновляет поездку
func (h *EventHandler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("UpdateEvent: неверный ID", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req eventdto.UpdateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("UpdateEvent: ошибка декодирования", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	log.Debug("UpdateEvent: данные запроса",
		"event_id", eventID,
		"user_id", userID,
		"title", req.Title,
		"description", req.Description,
		"start_date", req.StartDate,
		"is_public", req.IsPublic,
	)

	updateReq := &models.UpdateEventRequest{
		Title:       req.Title,
		Description: req.Description,
		StartDate:   req.StartDate,
		IsPublic:    req.IsPublic,
	}

	event, err := h.eventService.UpdateEvent(r.Context(), eventID, *userID, updateReq)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEventNotFound):
			log.Debug("UpdateEvent: событие не найдено", "event_id", eventID)
			h.writeError(w, http.StatusNotFound, "event not found", "")
		case errors.Is(err, service.ErrNotOwner):
			log.Debug("UpdateEvent: пользователь не владелец", "user_id", userID, "event_id", eventID)
			h.writeError(w, http.StatusForbidden, "you are not the owner of this event", "")
		case errors.Is(err, service.ErrInvalidStartDate):
			log.Debug("UpdateEvent: дата начала в прошлом", "start_date", req.StartDate)
			h.writeError(w, http.StatusBadRequest, "start date cannot be in the past", "")
		default:
			log.Error("Ошибка обновления поездки", "error", err, "event_id", eventID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("UpdateEvent: поездка обновлена", "event_id", eventID, "user_id", userID)

	// Преобразуем в DTO
	response := eventdto.EventResponse{
		ID:          event.ID,
		RouteID:     event.RouteID,
		Title:       event.Title,
		Description: event.Description,
		OwnerID:     event.OwnerID,
		StartDate:   event.StartDate,
		EndDate:     event.EndDate,
		IsGroup:     event.IsGroup,
		Status:      event.Status,
		IsPublic:    event.IsPublic,
		CreatedAt:   event.CreatedAt,
		UpdatedAt:   event.UpdatedAt,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// DeleteEvent отменяет поездку
func (h *EventHandler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("DeleteEvent: неверный ID", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	err = h.eventService.DeleteEvent(r.Context(), eventID, *userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEventNotFound):
			h.writeError(w, http.StatusNotFound, "event not found", "")
		case errors.Is(err, service.ErrNotOwner):
			h.writeError(w, http.StatusForbidden, "you are not the owner of this event", "")
		default:
			log.Error("Ошибка отмены поездки", "error", err, "event_id", eventID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "event cancelled successfully"})
}

// JoinEvent запрос на участие в поездке (пользователь подаёт заявку)
func (h *EventHandler) JoinEvent(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("JoinEvent: неверный ID", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	err = h.eventService.JoinEvent(r.Context(), eventID, *userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEventNotFound):
			h.writeError(w, http.StatusNotFound, "event not found", "")
		case errors.Is(err, service.ErrAlreadyParticipant):
			h.writeError(w, http.StatusConflict, "already a participant", "")
		default:
			log.Error("Ошибка присоединения к поездке", "error", err, "event_id", eventID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "join request sent"})
}

// LeaveEvent выход из поездки
func (h *EventHandler) LeaveEvent(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("LeaveEvent: неверный ID", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	err = h.eventService.LeaveEvent(r.Context(), eventID, *userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEventNotFound):
			h.writeError(w, http.StatusNotFound, "event not found", "")
		default:
			if err.Error() == "owner cannot leave the event" {
				h.writeError(w, http.StatusBadRequest, "owner cannot leave the event", "")
			} else {
				log.Error("Ошибка выхода из поездки", "error", err, "event_id", eventID)
				h.writeError(w, http.StatusInternalServerError, "internal server error", "")
			}
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "left event successfully"})
}

// AcceptParticipant принимает участника (владелец утверждает заявку)
func (h *EventHandler) AcceptParticipant(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]
	participantIDStr := vars["userId"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("AcceptParticipant: неверный ID события", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	participantID, err := uuid.Parse(participantIDStr)
	if err != nil {
		log.Debug("AcceptParticipant: неверный ID участника", "participant_id", participantIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid participant id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	err = h.eventService.AcceptParticipant(r.Context(), eventID, *userID, participantID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEventNotFound):
			h.writeError(w, http.StatusNotFound, "event not found", "")
		case errors.Is(err, service.ErrNotOwner):
			h.writeError(w, http.StatusForbidden, "you are not the owner of this event", "")
		default:
			log.Error("Ошибка принятия участника", "error", err, "event_id", eventID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "participant accepted"})
}

// DeclineParticipant отклоняет участника (владелец отклоняет заявку)
func (h *EventHandler) DeclineParticipant(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]
	participantIDStr := vars["userId"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("DeclineParticipant: неверный ID события", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	participantID, err := uuid.Parse(participantIDStr)
	if err != nil {
		log.Debug("DeclineParticipant: неверный ID участника", "participant_id", participantIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid participant id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	err = h.eventService.DeclineParticipant(r.Context(), eventID, *userID, participantID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEventNotFound):
			h.writeError(w, http.StatusNotFound, "event not found", "")
		case errors.Is(err, service.ErrNotOwner):
			h.writeError(w, http.StatusForbidden, "you are not the owner of this event", "")
		default:
			log.Error("Ошибка отклонения участника", "error", err, "event_id", eventID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "participant declined"})
}

// GetParticipants возвращает список участников
func (h *EventHandler) GetParticipants(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("GetParticipants: неверный ID", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	participants, err := h.eventService.GetParticipants(r.Context(), eventID)
	if err != nil {
		log.Error("Ошибка получения участников", "error", err, "event_id", eventID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	response := make([]eventdto.ParticipantResponse, len(participants))
	for i, p := range participants {
		response[i] = eventdto.ParticipantResponse{
			UserID:          p.UserID,
			Role:            p.Role,
			ApprovedByOwner: p.ApprovedByOwner,
			ApprovedByUser:  p.ApprovedByUser,
			InvitedEmail:    p.InvitedEmail,
			JoinedAt:        p.JoinedAt,
		}
	}

	h.writeJSON(w, http.StatusOK, response)
}

// GetInvites возвращает список приглашений текущего пользователя
func (h *EventHandler) GetInvites(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("GetInvites: запрос приглашений")

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		log.Debug("GetInvites: пользователь не авторизован")
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	log.Debug("GetInvites: ищем приглашения для пользователя", "user_id", userID)

	invites, err := h.eventService.GetInvitesByUserID(r.Context(), *userID)
	if err != nil {
		log.Error("Ошибка получения приглашений", "error", err, "user_id", userID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	// Преобразуем в DTO
	response := make([]eventdto.InviteResponse, 0, len(invites))
	for _, inv := range invites {
		resp := eventdto.InviteResponse{
			InviteID:          inv.InviteID,
			EventID:           inv.EventID,
			EventTitle:        inv.EventTitle,
			EventStartDate:    inv.EventStartDate,
			EventEndDate:      inv.EventEndDate,
			EventOwnerID:      inv.EventOwnerID,
			ParticipantsCount: inv.ParticipantsCount,
			InvitedBy:         inv.InvitedBy,
			InvitedAt:         inv.InvitedAt,
		}
		if inv.EventDescription != "" {
			resp.EventDescription = inv.EventDescription
		}
		response = append(response, resp)
	}

	log.Info("GetInvites: приглашения получены", "user_id", userID, "count", len(response))
	h.writeJSON(w, http.StatusOK, response)
}

// AcceptInvite принимает приглашение по invite_id
func (h *EventHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	inviteIDStr := vars["invite_id"]

	inviteID, err := uuid.Parse(inviteIDStr)
	if err != nil {
		log.Debug("AcceptInvite: неверный ID приглашения", "invite_id", inviteIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid invite id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	log.Debug("AcceptInvite: принятие приглашения", "invite_id", inviteID, "user_id", userID)

	if err := h.eventService.AcceptInvite(r.Context(), inviteID, *userID); err != nil {
		switch {
		case errors.Is(err, service.ErrInviteNotFound):
			log.Debug("AcceptInvite: приглашение не найдено", "invite_id", inviteID)
			h.writeError(w, http.StatusNotFound, "invitation not found", "")
		case errors.Is(err, service.ErrInviteAlreadyProcessed):
			log.Debug("AcceptInvite: приглашение уже обработано", "invite_id", inviteID)
			h.writeError(w, http.StatusConflict, "invitation already processed", "")
		default:
			log.Error("Ошибка принятия приглашения", "error", err, "invite_id", inviteID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("AcceptInvite: приглашение принято", "invite_id", inviteID, "user_id", userID)
	h.writeJSON(w, http.StatusOK, map[string]string{"message": "invitation accepted successfully"})
}

// DeclineInvite отклоняет приглашение по invite_id
func (h *EventHandler) DeclineInvite(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	inviteIDStr := vars["invite_id"]

	inviteID, err := uuid.Parse(inviteIDStr)
	if err != nil {
		log.Debug("DeclineInvite: неверный ID приглашения", "invite_id", inviteIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid invite id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	log.Debug("DeclineInvite: отклонение приглашения", "invite_id", inviteID, "user_id", userID)

	if err := h.eventService.DeclineInvite(r.Context(), inviteID, *userID); err != nil {
		switch {
		case errors.Is(err, service.ErrInviteNotFound):
			log.Debug("DeclineInvite: приглашение не найдено", "invite_id", inviteID)
			h.writeError(w, http.StatusNotFound, "invitation not found", "")
		case errors.Is(err, service.ErrInviteAlreadyProcessed):
			log.Debug("DeclineInvite: приглашение уже обработано", "invite_id", inviteID)
			h.writeError(w, http.StatusConflict, "invitation already processed", "")
		default:
			log.Error("Ошибка отклонения приглашения", "error", err, "invite_id", inviteID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("DeclineInvite: приглашение отклонено", "invite_id", inviteID, "user_id", userID)
	h.writeJSON(w, http.StatusOK, map[string]string{"message": "invitation declined successfully"})
}

// InviteParticipants владелец приглашает новых участников
func (h *EventHandler) InviteParticipants(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("InviteParticipants: неверный ID события", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req eventdto.InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("InviteParticipants: ошибка декодирования", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if len(req.Emails) == 0 {
		h.writeError(w, http.StatusBadRequest, "emails list is required", "")
		return
	}

	if err := h.eventService.InviteParticipants(r.Context(), eventID, *userID, req.Emails); err != nil {
		switch {
		case errors.Is(err, service.ErrEventNotFound):
			h.writeError(w, http.StatusNotFound, "event not found", "")
		case errors.Is(err, service.ErrNotOwner):
			h.writeError(w, http.StatusForbidden, "you are not the owner of this event", "")
		default:
			log.Error("Ошибка приглашения участников", "error", err, "event_id", eventID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "invitations sent successfully"})
}

// GetWaypoints возвращает точки маршрута
func (h *EventHandler) GetWaypoints(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("GetWaypoints: неверный ID", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	waypoints, err := h.eventService.GetWaypoints(r.Context(), eventID)
	if err != nil {
		log.Error("Ошибка получения точек", "error", err, "event_id", eventID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	response := make([]eventdto.WaypointResponse, len(waypoints))
	for i, w := range waypoints {
		response[i] = eventdto.WaypointResponse{
			ID:           w.ID,
			City:         w.City,
			Description:  w.Description,
			Latitude:     w.Latitude,
			Longitude:    w.Longitude,
			PlannedStart: w.PlannedStart,
			PlannedEnd:   w.PlannedEnd,
			ActualStart:  w.ActualStart,
			ActualEnd:    w.ActualEnd,
			Status:       w.Status,
			OrderIndex:   w.OrderIndex,
		}
	}

	h.writeJSON(w, http.StatusOK, response)
}

// VisitWaypoint отмечает точку посещённой
func (h *EventHandler) VisitWaypoint(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]
	waypointIDStr := vars["waypointId"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("VisitWaypoint: неверный ID события", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	waypointID, err := uuid.Parse(waypointIDStr)
	if err != nil {
		log.Debug("VisitWaypoint: неверный ID точки", "waypoint_id", waypointIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid waypoint id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req eventdto.VisitWaypointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("VisitWaypoint: ошибка декодирования", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	visitReq := &models.VisitWaypointRequest{
		ActualStart: req.ActualStart,
		ActualEnd:   req.ActualEnd,
	}

	err = h.eventService.VisitWaypoint(r.Context(), eventID, waypointID, *userID, visitReq)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEventNotFound):
			h.writeError(w, http.StatusNotFound, "event not found", "")
		case errors.Is(err, service.ErrNotParticipant):
			h.writeError(w, http.StatusForbidden, "you are not a participant of this event", "")
		case errors.Is(err, service.ErrInvalidWaypoint):
			h.writeError(w, http.StatusNotFound, "waypoint not found", "")
		default:
			log.Error("Ошибка отметки точки", "error", err, "event_id", eventID, "waypoint_id", waypointID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "waypoint marked as visited"})
}

// AddMedia добавляет медиа к точке
func (h *EventHandler) AddMedia(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]
	waypointIDStr := vars["waypointId"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("AddMedia: неверный ID события", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	waypointID, err := uuid.Parse(waypointIDStr)
	if err != nil {
		log.Debug("AddMedia: неверный ID точки", "waypoint_id", waypointIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid waypoint id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req eventdto.AddMediaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("AddMedia: ошибка декодирования", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	addReq := &models.AddMediaRequest{
		MediaURL:     req.MediaURL,
		ThumbnailURL: req.ThumbnailURL,
		MediumURL:    req.MediumURL,
		MediaType:    req.MediaType,
		Description:  req.Description,
	}

	media, err := h.eventService.AddMedia(r.Context(), eventID, waypointID, *userID, addReq)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEventNotFound):
			h.writeError(w, http.StatusNotFound, "event not found", "")
		case errors.Is(err, service.ErrNotParticipant):
			h.writeError(w, http.StatusForbidden, "you are not a participant of this event", "")
		default:
			log.Error("Ошибка добавления медиа", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	response := eventdto.MediaResponse{
		ID:           media.ID,
		UserID:       media.UserID,
		MediaURL:     media.MediaURL,
		ThumbnailURL: media.ThumbnailURL,
		MediumURL:    media.MediumURL,
		MediaType:    media.MediaType,
		Description:  media.Description,
		CreatedAt:    media.CreatedAt,
	}

	h.writeJSON(w, http.StatusCreated, response)
}

// GetMedia возвращает медиа точки
func (h *EventHandler) GetMedia(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	waypointIDStr := vars["waypointId"]

	waypointID, err := uuid.Parse(waypointIDStr)
	if err != nil {
		log.Debug("GetMedia: неверный ID точки", "waypoint_id", waypointIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid waypoint id", "")
		return
	}

	pageStr := r.URL.Query().Get("page")
	sizeStr := r.URL.Query().Get("size")

	var page, size int

	if pageStr == "" && sizeStr == "" {
		page = 1
		size = 10000
		log.Debug("GetMedia: пагинация не указана, возвращаем все записи")
	} else {
		page, _ = strconv.Atoi(pageStr)
		if page < 1 {
			page = 1
		}
		size, _ = strconv.Atoi(sizeStr)
		if size < 1 || size > 100 {
			size = 20
		}
	}

	mediaList, total, err := h.eventService.GetMedia(r.Context(), waypointID, page, size)
	if err != nil {
		log.Error("Ошибка получения медиа", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	items := make([]eventdto.MediaResponse, len(mediaList))
	for i, m := range mediaList {
		items[i] = eventdto.MediaResponse{
			ID:           m.ID,
			UserID:       m.UserID,
			MediaURL:     m.MediaURL,
			ThumbnailURL: m.ThumbnailURL,
			MediumURL:    m.MediumURL,
			MediaType:    m.MediaType,
			Description:  m.Description,
			CreatedAt:    m.CreatedAt,
		}
	}

	response := map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// DeleteMedia удаляет медиа
func (h *EventHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	mediaIDStr := vars["mediaId"]

	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		log.Debug("DeleteMedia: неверный ID медиа", "media_id", mediaIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid media id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	err = h.eventService.DeleteMedia(r.Context(), mediaID, *userID)
	if err != nil {
		log.Error("Ошибка удаления медиа", "error", err, "media_id", mediaID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "media deleted successfully"})
}

// AddComment добавляет комментарий
func (h *EventHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	eventIDStr := vars["id"]
	waypointIDStr := vars["waypointId"]

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		log.Debug("AddComment: неверный ID события", "event_id", eventIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid event id", "")
		return
	}

	waypointID, err := uuid.Parse(waypointIDStr)
	if err != nil {
		log.Debug("AddComment: неверный ID точки", "waypoint_id", waypointIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid waypoint id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req eventdto.AddCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("AddComment: ошибка декодирования", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	comment, err := h.eventService.AddComment(r.Context(), eventID, waypointID, *userID, req.Text)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEventNotFound):
			h.writeError(w, http.StatusNotFound, "event not found", "")
		case errors.Is(err, service.ErrNotParticipant):
			h.writeError(w, http.StatusForbidden, "you are not a participant of this event", "")
		default:
			log.Error("Ошибка добавления комментария", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	response := eventdto.CommentResponse{
		ID:        comment.ID,
		UserID:    comment.UserID,
		Text:      comment.Text,
		CreatedAt: comment.CreatedAt,
		UpdatedAt: comment.UpdatedAt,
	}

	h.writeJSON(w, http.StatusCreated, response)
}

// UpdateComment обновляет комментарий
func (h *EventHandler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	commentIDStr := vars["commentId"]

	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		log.Debug("UpdateComment: неверный ID комментария", "comment_id", commentIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid comment id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req eventdto.UpdateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("UpdateComment: ошибка декодирования", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	err = h.eventService.UpdateComment(r.Context(), commentID, *userID, req.Text)
	if err != nil {
		log.Error("Ошибка обновления комментария", "error", err, "comment_id", commentID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "comment updated successfully"})
}

// DeleteComment удаляет комментарий
func (h *EventHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	commentIDStr := vars["commentId"]

	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		log.Debug("DeleteComment: неверный ID комментария", "comment_id", commentIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid comment id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	err = h.eventService.DeleteComment(r.Context(), commentID, *userID)
	if err != nil {
		log.Error("Ошибка удаления комментария", "error", err, "comment_id", commentID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "comment deleted successfully"})
}
