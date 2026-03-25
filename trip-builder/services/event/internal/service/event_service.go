package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/client/auth"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/client/route"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

var (
	ErrEventNotFound      = errors.New("event not found")
	ErrNotOwner           = errors.New("user is not the owner of this event")
	ErrNotParticipant     = errors.New("user is not a participant of this event")
	ErrAlreadyParticipant = errors.New("user is already a participant")
	ErrEventNotActive     = errors.New("event is not active")
	ErrEventAlreadyEnded  = errors.New("event has already ended")
	ErrInvalidStartDate   = errors.New("start date cannot be in the past")
	ErrInvalidWaypoint    = errors.New("invalid waypoint")
)

type EventService struct {
	eventRepo       repository.EventRepository
	participantRepo repository.ParticipantRepository
	waypointRepo    repository.WaypointRepository
	mediaRepo       repository.MediaRepository
	commentRepo     repository.CommentRepository
	routeClient     *route.Client
	authClient      *auth.Client
	producer        *kafka.EventProducer
	log             *logger.Logger
}

func NewEventService(
	eventRepo repository.EventRepository,
	participantRepo repository.ParticipantRepository,
	waypointRepo repository.WaypointRepository,
	mediaRepo repository.MediaRepository,
	commentRepo repository.CommentRepository,
	routeClient *route.Client,
	authClient *auth.Client,
	producer *kafka.EventProducer,
	log *logger.Logger,
) *EventService {
	return &EventService{
		eventRepo:       eventRepo,
		participantRepo: participantRepo,
		waypointRepo:    waypointRepo,
		mediaRepo:       mediaRepo,
		commentRepo:     commentRepo,
		routeClient:     routeClient,
		authClient:      authClient,
		producer:        producer,
		log:             log,
	}
}

// CreateEvent создаёт новую поездку
func (s *EventService) CreateEvent(ctx context.Context, userID uuid.UUID, req *models.CreateEventRequest) (*models.EventWithDetails, error) {
	s.log.Debug("CreateEvent: начало", "user_id", userID, "route_id", req.RouteID, "title", req.Title, "start_date", req.StartDate)
	s.log.Trace("CreateEvent: детали запроса", "is_group", req.IsGroup, "invited_emails_count", len(req.InvitedEmails))

	// Проверяем дату начала
	s.log.Trace("CreateEvent: проверка даты начала", "start_date", req.StartDate, "now", time.Now())
	if req.StartDate.Before(time.Now()) {
		s.log.Debug("CreateEvent: дата начала в прошлом", "start_date", req.StartDate, "now", time.Now())
		return nil, ErrInvalidStartDate
	}
	s.log.Trace("CreateEvent: дата начала валидна")

	// Получаем маршрут из Route Service
	s.log.Trace("CreateEvent: запрос маршрута из Route Service", "route_id", req.RouteID)
	routeData, err := s.routeClient.GetRouteWithWaypoints(ctx, req.RouteID.String())
	if err != nil {
		s.log.Error("CreateEvent: ошибка получения маршрута", "error", err, "route_id", req.RouteID)
		return nil, fmt.Errorf("failed to get route: %w", err)
	}
	s.log.Trace("CreateEvent: маршрут получен", "route_title", routeData.Title, "waypoints_count", len(routeData.Waypoints))

	// Создаём ID события
	eventID := uuid.New()
	now := time.Now()
	s.log.Trace("CreateEvent: создание события", "event_id", eventID)

	// Рассчитываем даты точек
	s.log.Trace("CreateEvent: расчёт дат точек", "start_date", req.StartDate, "waypoints_count", len(routeData.Waypoints))
	waypoints := s.calculateWaypointDates(routeData.Waypoints, req.StartDate, eventID)
	s.log.Trace("CreateEvent: точки рассчитаны", "waypoints_count", len(waypoints))

	// Вычисляем дату окончания
	endDate := req.StartDate
	if len(waypoints) > 0 && waypoints[len(waypoints)-1].PlannedEnd != nil {
		endDate = *waypoints[len(waypoints)-1].PlannedEnd
		s.log.Trace("CreateEvent: дата окончания вычислена", "end_date", endDate)
	}

	// Создаём событие
	event := &models.Event{
		ID:          eventID,
		RouteID:     req.RouteID,
		Title:       req.Title,
		Description: req.Description,
		OwnerID:     userID,
		StartDate:   req.StartDate,
		EndDate:     endDate,
		IsGroup:     req.IsGroup,
		Status:      "planned",
		IsPublic:    false,
		CreatedAt:   now,
	}

	// Создаём событие и точки в одной транзакции
	if err := s.eventRepo.CreateWithWaypoints(ctx, event, waypoints); err != nil {
		s.log.Error("CreateEvent: ошибка создания события с точками", "error", err, "event_id", eventID)
		return nil, err
	}
	s.log.Trace("CreateEvent: событие и точки сохранены в БД", "event_id", eventID, "waypoints_count", len(waypoints))

	// Добавляем владельца как участника
	participant := &models.Participant{
		InviteID: uuid.New(),
		EventID:  eventID,
		UserID:   userID,
		Role:     "owner",
		Status:   "accepted",
	}
	if err := s.participantRepo.Add(ctx, participant); err != nil {
		s.log.Error("CreateEvent: ошибка добавления владельца", "error", err, "event_id", eventID)
	} else {
		s.log.Trace("CreateEvent: владелец добавлен как участник", "event_id", eventID, "user_id", userID)
	}

	// Обрабатываем приглашения
	participants := []uuid.UUID{userID}
	if req.IsGroup && len(req.InvitedEmails) > 0 {
		s.log.Trace("CreateEvent: обработка приглашений", "invited_emails", req.InvitedEmails)

		for _, email := range req.InvitedEmails {
			// Проверяем, существует ли пользователь с таким email
			userInfo, err := s.authClient.GetUserByEmail(ctx, email)
			if err != nil {
				s.log.Error("CreateEvent: ошибка проверки пользователя", "error", err, "email", email)
				continue
			}

			var invite *models.Participant

			if userInfo != nil {
				// Пользователь уже зарегистрирован — добавляем сразу с user_id
				invite = &models.Participant{
					InviteID:     uuid.New(),
					EventID:      eventID,
					UserID:       userInfo.UserID,
					Role:         "member",
					Status:       "pending",
					InvitedEmail: "",
				}
				s.log.Trace("CreateEvent: пользователь найден, создаём участника",
					"event_id", eventID, "user_id", userInfo.UserID, "email", email)

				// Добавляем в список участников для Kafka события
				participants = append(participants, userInfo.UserID)
			} else {
				// Пользователь не зарегистрирован — создаём приглашение
				invite = &models.Participant{
					InviteID:     uuid.New(),
					EventID:      eventID,
					UserID:       uuid.Nil,
					Role:         "member",
					Status:       "pending",
					InvitedEmail: email,
				}
				s.log.Trace("CreateEvent: пользователь не найден, создаём приглашение",
					"event_id", eventID, "email", email)
			}

			if err := s.participantRepo.Add(ctx, invite); err != nil {
				s.log.Error("CreateEvent: ошибка создания", "error", err, "email", email)
			}
		}
	}

	// Публикуем событие
	s.log.Trace("CreateEvent: публикация события в Kafka", "event_id", eventID)
	if err := s.producer.PublishEventCreated(ctx, eventID, req.RouteID, userID, req.Title, req.StartDate, endDate, participants); err != nil {
		s.log.Error("CreateEvent: ошибка публикации события", "error", err, "event_id", eventID)
	} else {
		s.log.Trace("CreateEvent: событие опубликовано", "event_id", eventID)
	}

	s.log.Info("CreateEvent: поездка создана", "event_id", eventID, "user_id", userID)

	// Формируем ответ
	result := &models.EventWithDetails{
		Event:        *event,
		Participants: []models.Participant{*participant},
		Waypoints:    waypoints,
	}
	s.log.Debug("CreateEvent: завершено", "event_id", eventID, "waypoints_count", len(waypoints), "participants_count", len(result.Participants))
	return result, nil
}

// GetEventByID получает поездку по ID
func (s *EventService) GetEventByID(ctx context.Context, eventID uuid.UUID, userID *uuid.UUID) (*models.EventWithDetails, error) {
	s.log.Debug("GetEventByID: начало", "event_id", eventID, "user_id", userID)

	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		s.log.Error("GetEventByID: ошибка получения события", "error", err, "event_id", eventID)
		return nil, err
	}
	if event == nil {
		s.log.Debug("GetEventByID: событие не найдено", "event_id", eventID)
		return nil, ErrEventNotFound
	}
	s.log.Trace("GetEventByID: событие получено", "event_id", eventID, "title", event.Title, "is_public", event.IsPublic)

	// Проверяем права доступа
	if !event.IsPublic {
		s.log.Trace("GetEventByID: событие приватное, проверка прав", "event_id", eventID, "user_id", userID)
		if userID == nil {
			s.log.Debug("GetEventByID: попытка доступа к приватному событию без авторизации", "event_id", eventID)
			return nil, ErrEventNotFound
		}
		isParticipant, err := s.participantRepo.IsParticipant(ctx, eventID, *userID)
		if err != nil {
			s.log.Error("GetEventByID: ошибка проверки участия", "error", err, "event_id", eventID, "user_id", userID)
			return nil, err
		}
		if !isParticipant {
			s.log.Debug("GetEventByID: пользователь не участник приватного события", "event_id", eventID, "user_id", userID)
			return nil, ErrEventNotFound
		}
		s.log.Trace("GetEventByID: пользователь участник приватного события", "event_id", eventID, "user_id", userID)
	} else {
		s.log.Trace("GetEventByID: событие публичное, доступ разрешён", "event_id", eventID)
	}

	// Получаем участников
	s.log.Trace("GetEventByID: получение участников", "event_id", eventID)
	participants, err := s.participantRepo.GetByEventID(ctx, eventID)
	if err != nil {
		s.log.Error("GetEventByID: ошибка получения участников", "error", err, "event_id", eventID)
		participants = []models.Participant{}
	} else {
		s.log.Trace("GetEventByID: участники получены", "event_id", eventID, "count", len(participants))
	}

	// Получаем точки
	s.log.Trace("GetEventByID: получение точек маршрута", "event_id", eventID)
	waypoints, err := s.waypointRepo.GetByEventID(ctx, eventID)
	if err != nil {
		s.log.Error("GetEventByID: ошибка получения точек", "error", err, "event_id", eventID)
		waypoints = []models.Waypoint{}
	} else {
		s.log.Trace("GetEventByID: точки получены", "event_id", eventID, "count", len(waypoints))
	}

	result := &models.EventWithDetails{
		Event:        *event,
		Participants: participants,
		Waypoints:    waypoints,
	}
	s.log.Debug("GetEventByID: завершено", "event_id", eventID, "participants_count", len(participants), "waypoints_count", len(waypoints))
	return result, nil
}

// GetMyEvents получает поездки пользователя (где он является участником)
func (s *EventService) GetMyEvents(ctx context.Context, userID uuid.UUID, status string, page, size int) ([]models.Event, int64, error) {
	s.log.Debug("GetMyEvents: начало", "user_id", userID, "status", status, "page", page, "size", size)

	offset := (page - 1) * size
	s.log.Trace("GetMyEvents: параметры запроса", "limit", size, "offset", offset)

	events, total, err := s.eventRepo.ListByUserID(ctx, userID, status, size, offset)
	if err != nil {
		s.log.Error("GetMyEvents: ошибка получения поездок", "error", err, "user_id", userID)
		return nil, 0, err
	}

	s.log.Debug("GetMyEvents: завершено", "user_id", userID, "count", len(events), "total", total)
	return events, total, nil
}

// GetPublicEvents получает публичные завершённые поездки
func (s *EventService) GetPublicEvents(ctx context.Context, page, size int) ([]models.Event, int64, error) {
	s.log.Debug("GetPublicEvents: начало", "page", page, "size", size)

	offset := (page - 1) * size
	s.log.Trace("GetPublicEvents: параметры запроса", "limit", size, "offset", offset)

	events, total, err := s.eventRepo.ListPublic(ctx, size, offset)
	if err != nil {
		s.log.Error("GetPublicEvents: ошибка получения публичных поездок", "error", err)
		return nil, 0, err
	}

	s.log.Debug("GetPublicEvents: завершено", "count", len(events), "total", total)
	return events, total, nil
}

// UpdateEvent обновляет поездку
func (s *EventService) UpdateEvent(ctx context.Context, eventID, userID uuid.UUID, req *models.UpdateEventRequest) (*models.Event, error) {
	s.log.Debug("UpdateEvent: начало", "event_id", eventID, "user_id", userID)
	s.log.Trace("UpdateEvent: данные запроса", "title", req.Title, "description", req.Description, "is_public", req.IsPublic, "start_date", req.StartDate)

	// Проверяем права
	s.log.Trace("UpdateEvent: проверка прав владельца", "event_id", eventID, "user_id", userID)
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, userID)
	if err != nil {
		s.log.Error("UpdateEvent: ошибка проверки владельца", "error", err, "event_id", eventID, "user_id", userID)
		return nil, err
	}
	if !isOwner {
		s.log.Debug("UpdateEvent: пользователь не владелец", "event_id", eventID, "user_id", userID)
		return nil, ErrNotOwner
	}
	s.log.Trace("UpdateEvent: пользователь владелец", "event_id", eventID, "user_id", userID)

	// Получаем событие
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		s.log.Error("UpdateEvent: ошибка получения события", "error", err, "event_id", eventID)
		return nil, err
	}
	if event == nil {
		s.log.Debug("UpdateEvent: событие не найдено", "event_id", eventID)
		return nil, ErrEventNotFound
	}
	s.log.Trace("UpdateEvent: событие получено", "event_id", eventID, "title", event.Title)

	// Обновляем поля
	if req.Title != nil {
		s.log.Trace("UpdateEvent: обновление title", "old", event.Title, "new", *req.Title)
		event.Title = *req.Title
	}
	if req.Description != nil {
		s.log.Trace("UpdateEvent: обновление description", "old", event.Description, "new", *req.Description)
		event.Description = *req.Description
	}
	if req.IsPublic != nil {
		s.log.Trace("UpdateEvent: обновление is_public", "old", event.IsPublic, "new", *req.IsPublic)
		event.IsPublic = *req.IsPublic
	}

	// Если меняется дата начала, нужно пересчитать точки
	if req.StartDate != nil && !req.StartDate.Equal(event.StartDate) {
		s.log.Trace("UpdateEvent: обновление start_date", "old", event.StartDate, "new", *req.StartDate)
		if req.StartDate.Before(time.Now()) {
			s.log.Debug("UpdateEvent: дата начала в прошлом", "start_date", *req.StartDate)
			return nil, ErrInvalidStartDate
		}
		event.StartDate = *req.StartDate

		// Пересчитываем даты точек
		waypoints, err := s.waypointRepo.GetByEventID(ctx, eventID)
		if err != nil {
			s.log.Error("UpdateEvent: ошибка получения точек для пересчёта", "error", err, "event_id", eventID)
		} else {
			s.log.Trace("UpdateEvent: пересчёт дат точек", "event_id", eventID, "waypoints_count", len(waypoints))
			newWaypoints := s.recalculateWaypointDates(waypoints, event.StartDate)
			for i, wp := range newWaypoints {
				if err := s.waypointRepo.Update(ctx, &wp); err != nil {
					s.log.Error("UpdateEvent: ошибка обновления точки", "error", err, "waypoint_id", wp.ID)
				} else {
					waypoints[i] = wp
				}
			}

			// Обновляем дату окончания
			if len(waypoints) > 0 && waypoints[len(waypoints)-1].PlannedEnd != nil {
				event.EndDate = *waypoints[len(waypoints)-1].PlannedEnd
				s.log.Trace("UpdateEvent: дата окончания обновлена", "end_date", event.EndDate)
			}
		}
	}

	// Сохраняем изменения
	s.log.Trace("UpdateEvent: сохранение изменений в БД", "event_id", eventID)
	if err := s.eventRepo.Update(ctx, event); err != nil {
		s.log.Error("UpdateEvent: ошибка обновления события", "error", err, "event_id", eventID)
		return nil, err
	}

	// Публикуем событие
	s.log.Trace("UpdateEvent: публикация события в Kafka", "event_id", eventID)
	if err := s.producer.PublishEventUpdated(ctx, eventID, userID, event.Title, event.StartDate, event.EndDate, event.IsPublic); err != nil {
		s.log.Error("UpdateEvent: ошибка публикации события", "error", err, "event_id", eventID)
	} else {
		s.log.Trace("UpdateEvent: событие опубликовано", "event_id", eventID)
	}

	s.log.Info("UpdateEvent: поездка обновлена", "event_id", eventID, "user_id", userID)
	s.log.Debug("UpdateEvent: завершено", "event_id", eventID, "title", event.Title)
	return event, nil
}

// DeleteEvent удаляет/отменяет поездку
func (s *EventService) DeleteEvent(ctx context.Context, eventID, userID uuid.UUID) error {
	s.log.Debug("DeleteEvent: начало", "event_id", eventID, "user_id", userID)

	// Проверяем права
	s.log.Trace("DeleteEvent: проверка прав владельца", "event_id", eventID, "user_id", userID)
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, userID)
	if err != nil {
		s.log.Error("DeleteEvent: ошибка проверки владельца", "error", err, "event_id", eventID, "user_id", userID)
		return err
	}
	if !isOwner {
		s.log.Debug("DeleteEvent: пользователь не владелец", "event_id", eventID, "user_id", userID)
		return ErrNotOwner
	}
	s.log.Trace("DeleteEvent: пользователь владелец", "event_id", eventID, "user_id", userID)

	// Получаем событие
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		s.log.Error("DeleteEvent: ошибка получения события", "error", err, "event_id", eventID)
		return err
	}
	if event == nil {
		s.log.Debug("DeleteEvent: событие не найдено", "event_id", eventID)
		return ErrEventNotFound
	}
	s.log.Trace("DeleteEvent: событие получено", "event_id", eventID, "status", event.Status)

	// Обновляем статус
	s.log.Trace("DeleteEvent: обновление статуса на cancelled", "event_id", eventID)
	if err := s.eventRepo.UpdateStatus(ctx, eventID, "cancelled"); err != nil {
		s.log.Error("DeleteEvent: ошибка обновления статуса", "error", err, "event_id", eventID)
		return err
	}

	// Публикуем событие
	s.log.Trace("DeleteEvent: публикация события в Kafka", "event_id", eventID)
	if err := s.producer.PublishEventCancelled(ctx, eventID, userID); err != nil {
		s.log.Error("DeleteEvent: ошибка публикации события", "error", err, "event_id", eventID)
	} else {
		s.log.Trace("DeleteEvent: событие опубликовано", "event_id", eventID)
	}

	s.log.Info("DeleteEvent: поездка отменена", "event_id", eventID, "user_id", userID)
	s.log.Debug("DeleteEvent: завершено", "event_id", eventID)
	return nil
}

// JoinEvent запрос на участие в поездке
func (s *EventService) JoinEvent(ctx context.Context, eventID, userID uuid.UUID) error {
	s.log.Debug("JoinEvent: начало", "event_id", eventID, "user_id", userID)

	// Проверяем существование события
	s.log.Trace("JoinEvent: проверка существования события", "event_id", eventID)
	exists, err := s.eventRepo.Exists(ctx, eventID)
	if err != nil {
		s.log.Error("JoinEvent: ошибка проверки существования", "error", err, "event_id", eventID)
		return err
	}
	if !exists {
		s.log.Debug("JoinEvent: событие не найдено", "event_id", eventID)
		return ErrEventNotFound
	}

	// Проверяем, не участник ли уже
	s.log.Trace("JoinEvent: проверка участия", "event_id", eventID, "user_id", userID)
	isParticipant, err := s.participantRepo.IsParticipant(ctx, eventID, userID)
	if err != nil {
		s.log.Error("JoinEvent: ошибка проверки участия", "error", err, "event_id", eventID, "user_id", userID)
		return err
	}
	if isParticipant {
		s.log.Debug("JoinEvent: пользователь уже участник", "event_id", eventID, "user_id", userID)
		return ErrAlreadyParticipant
	}
	s.log.Trace("JoinEvent: пользователь не является участником", "event_id", eventID, "user_id", userID)

	// Добавляем участника
	participant := &models.Participant{
		InviteID: uuid.New(),
		EventID:  eventID,
		UserID:   userID,
		Role:     "member",
		Status:   "pending",
	}
	s.log.Trace("JoinEvent: добавление участника", "event_id", eventID, "user_id", userID)
	if err := s.participantRepo.Add(ctx, participant); err != nil {
		s.log.Error("JoinEvent: ошибка добавления участника", "error", err, "event_id", eventID, "user_id", userID)
		return err
	}

	s.log.Info("JoinEvent: запрос на участие создан", "event_id", eventID, "user_id", userID)
	s.log.Debug("JoinEvent: завершено", "event_id", eventID, "user_id", userID)
	return nil
}

// LeaveEvent выход из поездки
func (s *EventService) LeaveEvent(ctx context.Context, eventID, userID uuid.UUID) error {
	s.log.Debug("LeaveEvent: начало", "event_id", eventID, "user_id", userID)

	// Проверяем, не владелец ли
	s.log.Trace("LeaveEvent: проверка владельца", "event_id", eventID, "user_id", userID)
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, userID)
	if err != nil {
		s.log.Error("LeaveEvent: ошибка проверки владельца", "error", err, "event_id", eventID, "user_id", userID)
		return err
	}
	if isOwner {
		s.log.Debug("LeaveEvent: владелец не может покинуть поездку", "event_id", eventID, "user_id", userID)
		return errors.New("owner cannot leave the event")
	}
	s.log.Trace("LeaveEvent: пользователь не владелец", "event_id", eventID, "user_id", userID)

	s.log.Trace("LeaveEvent: удаление участника", "event_id", eventID, "user_id", userID)
	if err := s.participantRepo.Remove(ctx, eventID, userID); err != nil {
		s.log.Error("LeaveEvent: ошибка удаления участника", "error", err, "event_id", eventID, "user_id", userID)
		return err
	}

	s.log.Info("LeaveEvent: пользователь покинул поездку", "event_id", eventID, "user_id", userID)
	s.log.Debug("LeaveEvent: завершено", "event_id", eventID, "user_id", userID)
	return nil
}

// AcceptParticipant принимает участника
func (s *EventService) AcceptParticipant(ctx context.Context, eventID, ownerID, participantID uuid.UUID) error {
	s.log.Debug("AcceptParticipant: начало", "event_id", eventID, "owner_id", ownerID, "participant_id", participantID)

	// Проверяем права
	s.log.Trace("AcceptParticipant: проверка прав владельца", "event_id", eventID, "owner_id", ownerID)
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, ownerID)
	if err != nil {
		s.log.Error("AcceptParticipant: ошибка проверки владельца", "error", err, "event_id", eventID, "owner_id", ownerID)
		return err
	}
	if !isOwner {
		s.log.Debug("AcceptParticipant: пользователь не владелец", "event_id", eventID, "owner_id", ownerID)
		return ErrNotOwner
	}
	s.log.Trace("AcceptParticipant: права подтверждены", "event_id", eventID, "owner_id", ownerID)

	s.log.Trace("AcceptParticipant: обновление статуса участника", "event_id", eventID, "participant_id", participantID)
	if err := s.participantRepo.UpdateStatus(ctx, eventID, participantID, "accepted"); err != nil {
		s.log.Error("AcceptParticipant: ошибка обновления статуса", "error", err, "event_id", eventID, "participant_id", participantID)
		return err
	}

	s.log.Info("AcceptParticipant: участник принят", "event_id", eventID, "participant_id", participantID)
	s.log.Debug("AcceptParticipant: завершено", "event_id", eventID)
	return nil
}

// DeclineParticipant отклоняет участника
func (s *EventService) DeclineParticipant(ctx context.Context, eventID, ownerID, participantID uuid.UUID) error {
	s.log.Debug("DeclineParticipant: начало", "event_id", eventID, "owner_id", ownerID, "participant_id", participantID)

	// Проверяем права
	s.log.Trace("DeclineParticipant: проверка прав владельца", "event_id", eventID, "owner_id", ownerID)
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, ownerID)
	if err != nil {
		s.log.Error("DeclineParticipant: ошибка проверки владельца", "error", err, "event_id", eventID, "owner_id", ownerID)
		return err
	}
	if !isOwner {
		s.log.Debug("DeclineParticipant: пользователь не владелец", "event_id", eventID, "owner_id", ownerID)
		return ErrNotOwner
	}
	s.log.Trace("DeclineParticipant: права подтверждены", "event_id", eventID, "owner_id", ownerID)

	s.log.Trace("DeclineParticipant: обновление статуса участника", "event_id", eventID, "participant_id", participantID)
	if err := s.participantRepo.UpdateStatus(ctx, eventID, participantID, "declined"); err != nil {
		s.log.Error("DeclineParticipant: ошибка обновления статуса", "error", err, "event_id", eventID, "participant_id", participantID)
		return err
	}

	s.log.Info("DeclineParticipant: участник отклонён", "event_id", eventID, "participant_id", participantID)
	s.log.Debug("DeclineParticipant: завершено", "event_id", eventID)
	return nil
}

// GetParticipants получает список участников
func (s *EventService) GetParticipants(ctx context.Context, eventID uuid.UUID) ([]models.Participant, error) {
	s.log.Debug("GetParticipants: начало", "event_id", eventID)

	participants, err := s.participantRepo.GetByEventID(ctx, eventID)
	if err != nil {
		s.log.Error("GetParticipants: ошибка получения участников", "error", err, "event_id", eventID)
		return nil, err
	}

	s.log.Debug("GetParticipants: завершено", "event_id", eventID, "count", len(participants))
	return participants, nil
}

// GetWaypoints получает точки маршрута
func (s *EventService) GetWaypoints(ctx context.Context, eventID uuid.UUID) ([]models.Waypoint, error) {
	s.log.Debug("GetWaypoints: начало", "event_id", eventID)

	waypoints, err := s.waypointRepo.GetByEventID(ctx, eventID)
	if err != nil {
		s.log.Error("GetWaypoints: ошибка получения точек", "error", err, "event_id", eventID)
		return nil, err
	}

	s.log.Debug("GetWaypoints: завершено", "event_id", eventID, "count", len(waypoints))
	return waypoints, nil
}

// VisitWaypoint отмечает точку посещённой
func (s *EventService) VisitWaypoint(ctx context.Context, eventID, waypointID, userID uuid.UUID, req *models.VisitWaypointRequest) error {
	s.log.Debug("VisitWaypoint: начало", "event_id", eventID, "waypoint_id", waypointID, "user_id", userID)

	// Проверяем, что пользователь участник
	s.log.Trace("VisitWaypoint: проверка участия", "event_id", eventID, "user_id", userID)
	isParticipant, err := s.participantRepo.IsParticipant(ctx, eventID, userID)
	if err != nil {
		s.log.Error("VisitWaypoint: ошибка проверки участия", "error", err, "event_id", eventID, "user_id", userID)
		return err
	}
	if !isParticipant {
		s.log.Debug("VisitWaypoint: пользователь не участник", "event_id", eventID, "user_id", userID)
		return ErrNotParticipant
	}
	s.log.Trace("VisitWaypoint: пользователь участник", "event_id", eventID, "user_id", userID)

	// Получаем точку
	s.log.Trace("VisitWaypoint: получение точки", "waypoint_id", waypointID)
	waypoint, err := s.waypointRepo.GetByID(ctx, waypointID)
	if err != nil {
		s.log.Error("VisitWaypoint: ошибка получения точки", "error", err, "waypoint_id", waypointID)
		return err
	}
	if waypoint == nil {
		s.log.Debug("VisitWaypoint: точка не найдена", "waypoint_id", waypointID)
		return ErrInvalidWaypoint
	}
	s.log.Trace("VisitWaypoint: точка получена", "waypoint_id", waypointID, "city", waypoint.City)

	// Обновляем статус
	actualStart := req.ActualStart
	actualEnd := req.ActualEnd
	s.log.Trace("VisitWaypoint: обновление статуса точки", "waypoint_id", waypointID, "actual_start", actualStart, "actual_end", actualEnd)

	if err := s.waypointRepo.UpdateVisit(ctx, waypointID, actualStart, actualEnd); err != nil {
		s.log.Error("VisitWaypoint: ошибка обновления точки", "error", err, "waypoint_id", waypointID)
		return err
	}

	// Публикуем событие
	s.log.Trace("VisitWaypoint: публикация события в Kafka", "event_id", eventID, "waypoint_id", waypointID)
	if err := s.producer.PublishWaypointVisited(ctx, eventID, waypointID, userID, waypoint.City); err != nil {
		s.log.Error("VisitWaypoint: ошибка публикации события", "error", err, "event_id", eventID, "waypoint_id", waypointID)
	} else {
		s.log.Trace("VisitWaypoint: событие опубликовано", "event_id", eventID, "waypoint_id", waypointID)
	}

	s.log.Info("VisitWaypoint: точка отмечена как посещённая", "event_id", eventID, "waypoint_id", waypointID, "user_id", userID)
	s.log.Debug("VisitWaypoint: завершено", "event_id", eventID, "waypoint_id", waypointID)
	return nil
}

// AddMedia добавляет медиа к точке
func (s *EventService) AddMedia(ctx context.Context, eventID, waypointID, userID uuid.UUID, req *models.AddMediaRequest) (*models.Media, error) {
	s.log.Debug("AddMedia: начало", "event_id", eventID, "waypoint_id", waypointID, "user_id", userID)

	// Проверяем, что пользователь участник
	s.log.Trace("AddMedia: проверка участия", "event_id", eventID, "user_id", userID)
	isParticipant, err := s.participantRepo.IsParticipant(ctx, eventID, userID)
	if err != nil {
		s.log.Error("AddMedia: ошибка проверки участия", "error", err, "event_id", eventID, "user_id", userID)
		return nil, err
	}
	if !isParticipant {
		s.log.Debug("AddMedia: пользователь не участник", "event_id", eventID, "user_id", userID)
		return nil, ErrNotParticipant
	}
	s.log.Trace("AddMedia: пользователь участник", "event_id", eventID, "user_id", userID)

	// Создаём медиа
	mediaID := uuid.New()
	s.log.Trace("AddMedia: создание медиа", "media_id", mediaID, "media_type", req.MediaType, "media_url", req.MediaURL)

	media := &models.Media{
		ID:           mediaID,
		EventID:      eventID,
		WaypointID:   waypointID,
		UserID:       userID,
		MediaURL:     req.MediaURL,
		ThumbnailURL: req.ThumbnailURL,
		MediumURL:    req.MediumURL,
		MediaType:    req.MediaType,
		Description:  req.Description,
		CreatedAt:    time.Now(),
	}

	if err := s.mediaRepo.Create(ctx, media); err != nil {
		s.log.Error("AddMedia: ошибка создания медиа", "error", err, "media_id", mediaID)
		return nil, err
	}

	s.log.Info("AddMedia: медиа добавлено", "event_id", eventID, "waypoint_id", waypointID, "media_id", mediaID)
	s.log.Debug("AddMedia: завершено", "media_id", mediaID)
	return media, nil
}

// GetMedia получает медиа точки
func (s *EventService) GetMedia(ctx context.Context, waypointID uuid.UUID, page, size int) ([]models.Media, int64, error) {
	s.log.Debug("GetMedia: начало", "waypoint_id", waypointID, "page", page, "size", size)

	offset := (page - 1) * size
	s.log.Trace("GetMedia: параметры запроса", "limit", size, "offset", offset)

	mediaList, total, err := s.mediaRepo.GetByWaypointID(ctx, waypointID, size, offset)
	if err != nil {
		s.log.Error("GetMedia: ошибка получения медиа", "error", err, "waypoint_id", waypointID)
		return nil, 0, err
	}

	s.log.Debug("GetMedia: завершено", "waypoint_id", waypointID, "count", len(mediaList), "total", total)
	return mediaList, total, nil
}

// DeleteMedia удаляет медиа
func (s *EventService) DeleteMedia(ctx context.Context, mediaID, userID uuid.UUID) error {
	s.log.Debug("DeleteMedia: начало", "media_id", mediaID, "user_id", userID)

	// Проверяем права
	s.log.Trace("DeleteMedia: проверка прав автора", "media_id", mediaID, "user_id", userID)
	isOwner, err := s.mediaRepo.IsOwner(ctx, mediaID, userID)
	if err != nil {
		s.log.Error("DeleteMedia: ошибка проверки автора", "error", err, "media_id", mediaID, "user_id", userID)
		return err
	}
	if !isOwner {
		s.log.Trace("DeleteMedia: пользователь не автор, проверка владельца события", "media_id", mediaID, "user_id", userID)
		// Проверяем, может быть владелец поездки
		media, err := s.mediaRepo.GetByID(ctx, mediaID)
		if err != nil {
			s.log.Error("DeleteMedia: ошибка получения медиа", "error", err, "media_id", mediaID)
			return err
		}
		if media != nil {
			isEventOwner, err := s.participantRepo.IsOwner(ctx, media.EventID, userID)
			if err != nil {
				s.log.Error("DeleteMedia: ошибка проверки владельца события", "error", err, "event_id", media.EventID)
				return err
			}
			if !isEventOwner {
				s.log.Debug("DeleteMedia: пользователь не авторизован", "media_id", mediaID, "user_id", userID)
				return errors.New("not authorized to delete this media")
			}
			s.log.Trace("DeleteMedia: пользователь владелец события", "media_id", mediaID, "event_id", media.EventID)
		}
	} else {
		s.log.Trace("DeleteMedia: пользователь автор медиа", "media_id", mediaID, "user_id", userID)
	}

	s.log.Trace("DeleteMedia: удаление медиа", "media_id", mediaID)
	if err := s.mediaRepo.Delete(ctx, mediaID); err != nil {
		s.log.Error("DeleteMedia: ошибка удаления медиа", "error", err, "media_id", mediaID)
		return err
	}

	s.log.Info("DeleteMedia: медиа удалено", "media_id", mediaID, "user_id", userID)
	s.log.Debug("DeleteMedia: завершено", "media_id", mediaID)
	return nil
}

// AddComment добавляет комментарий
func (s *EventService) AddComment(ctx context.Context, eventID, waypointID, userID uuid.UUID, text string) (*models.Comment, error) {
	s.log.Debug("AddComment: начало", "event_id", eventID, "waypoint_id", waypointID, "user_id", userID)

	// Проверяем, что пользователь участник
	s.log.Trace("AddComment: проверка участия", "event_id", eventID, "user_id", userID)
	isParticipant, err := s.participantRepo.IsParticipant(ctx, eventID, userID)
	if err != nil {
		s.log.Error("AddComment: ошибка проверки участия", "error", err, "event_id", eventID, "user_id", userID)
		return nil, err
	}
	if !isParticipant {
		s.log.Debug("AddComment: пользователь не участник", "event_id", eventID, "user_id", userID)
		return nil, ErrNotParticipant
	}
	s.log.Trace("AddComment: пользователь участник", "event_id", eventID, "user_id", userID)

	// Создаём комментарий
	commentID := uuid.New()
	s.log.Trace("AddComment: создание комментария", "comment_id", commentID)

	comment := &models.Comment{
		ID:         commentID,
		EventID:    eventID,
		WaypointID: waypointID,
		UserID:     userID,
		Text:       text,
		CreatedAt:  time.Now(),
	}

	if err := s.commentRepo.Create(ctx, comment); err != nil {
		s.log.Error("AddComment: ошибка создания комментария", "error", err, "comment_id", commentID)
		return nil, err
	}

	s.log.Info("AddComment: комментарий добавлен", "event_id", eventID, "waypoint_id", waypointID, "comment_id", commentID)
	s.log.Debug("AddComment: завершено", "comment_id", commentID)
	return comment, nil
}

// UpdateComment обновляет комментарий
func (s *EventService) UpdateComment(ctx context.Context, commentID, userID uuid.UUID, text string) error {
	s.log.Debug("UpdateComment: начало", "comment_id", commentID, "user_id", userID)

	// Проверяем права
	s.log.Trace("UpdateComment: проверка прав автора", "comment_id", commentID, "user_id", userID)
	isAuthor, err := s.commentRepo.IsAuthor(ctx, commentID, userID)
	if err != nil {
		s.log.Error("UpdateComment: ошибка проверки автора", "error", err, "comment_id", commentID, "user_id", userID)
		return err
	}
	if !isAuthor {
		s.log.Debug("UpdateComment: пользователь не автор", "comment_id", commentID, "user_id", userID)
		return errors.New("not authorized to update this comment")
	}
	s.log.Trace("UpdateComment: пользователь автор", "comment_id", commentID, "user_id", userID)

	comment := &models.Comment{
		ID:   commentID,
		Text: text,
	}

	s.log.Trace("UpdateComment: обновление комментария", "comment_id", commentID)
	if err := s.commentRepo.Update(ctx, comment); err != nil {
		s.log.Error("UpdateComment: ошибка обновления комментария", "error", err, "comment_id", commentID)
		return err
	}

	s.log.Info("UpdateComment: комментарий обновлён", "comment_id", commentID, "user_id", userID)
	s.log.Debug("UpdateComment: завершено", "comment_id", commentID)
	return nil
}

// DeleteComment удаляет комментарий
func (s *EventService) DeleteComment(ctx context.Context, commentID, userID uuid.UUID) error {
	s.log.Debug("DeleteComment: начало", "comment_id", commentID, "user_id", userID)

	// Проверяем права
	s.log.Trace("DeleteComment: проверка прав автора", "comment_id", commentID, "user_id", userID)
	isAuthor, err := s.commentRepo.IsAuthor(ctx, commentID, userID)
	if err != nil {
		s.log.Error("DeleteComment: ошибка проверки автора", "error", err, "comment_id", commentID, "user_id", userID)
		return err
	}
	if !isAuthor {
		s.log.Trace("DeleteComment: пользователь не автор, проверка владельца события", "comment_id", commentID, "user_id", userID)
		// Проверяем, может быть владелец поездки
		comment, err := s.commentRepo.GetByID(ctx, commentID)
		if err != nil {
			s.log.Error("DeleteComment: ошибка получения комментария", "error", err, "comment_id", commentID)
			return err
		}
		if comment != nil {
			isEventOwner, err := s.participantRepo.IsOwner(ctx, comment.EventID, userID)
			if err != nil {
				s.log.Error("DeleteComment: ошибка проверки владельца события", "error", err, "event_id", comment.EventID)
				return err
			}
			if !isEventOwner {
				s.log.Debug("DeleteComment: пользователь не авторизован", "comment_id", commentID, "user_id", userID)
				return errors.New("not authorized to delete this comment")
			}
			s.log.Trace("DeleteComment: пользователь владелец события", "comment_id", commentID, "event_id", comment.EventID)
		}
	} else {
		s.log.Trace("DeleteComment: пользователь автор комментария", "comment_id", commentID, "user_id", userID)
	}

	s.log.Trace("DeleteComment: удаление комментария", "comment_id", commentID)
	if err := s.commentRepo.Delete(ctx, commentID); err != nil {
		s.log.Error("DeleteComment: ошибка удаления комментария", "error", err, "comment_id", commentID)
		return err
	}

	s.log.Info("DeleteComment: комментарий удалён", "comment_id", commentID, "user_id", userID)
	s.log.Debug("DeleteComment: завершено", "comment_id", commentID)
	return nil
}

// StartStatusUpdater запускает фоновое обновление статусов поездок
func (s *EventService) StartStatusUpdater(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.log.Info("StartStatusUpdater: запуск обновления статусов поездок", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			s.log.Info("StartStatusUpdater: остановка обновления статусов поездок")
			return
		case <-ticker.C:
			s.log.Trace("StartStatusUpdater: выполнение обновления статусов")
			s.updateEventStatuses(ctx)
		}
	}
}

func (s *EventService) completeEvent(ctx context.Context, eventID uuid.UUID) error {
	s.log.Trace("completeEvent: начало", "event_id", eventID)

	// Получаем событие
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		s.log.Error("completeEvent: ошибка получения события", "error", err, "event_id", eventID)
		return err
	}
	if event == nil {
		s.log.Trace("completeEvent: событие не найдено", "event_id", eventID)
		return nil
	}
	s.log.Trace("completeEvent: событие получено", "event_id", eventID, "status", event.Status)

	// Обновляем статус
	s.log.Trace("completeEvent: обновление статуса на completed", "event_id", eventID)
	if err := s.eventRepo.UpdateStatus(ctx, eventID, "completed"); err != nil {
		s.log.Error("completeEvent: ошибка обновления статуса", "error", err, "event_id", eventID)
		return err
	}

	// Получаем участников
	participants, err := s.participantRepo.GetByEventID(ctx, eventID)
	if err != nil {
		s.log.Error("completeEvent: ошибка получения участников", "error", err, "event_id", eventID)
	}

	participantIDs := make([]uuid.UUID, 0, len(participants))
	for _, p := range participants {
		if p.Status == "accepted" {
			participantIDs = append(participantIDs, p.UserID)
		}
	}
	s.log.Trace("completeEvent: участники для уведомления", "event_id", eventID, "count", len(participantIDs))

	// MVP0: получаем страну из маршрута
	// Для MVP1: собирать уникальные страны из посещённых точек
	routeData, err := s.routeClient.GetRouteWithWaypoints(ctx, event.RouteID.String())
	if err != nil {
		s.log.Error("completeEvent: ошибка получения маршрута для определения страны", "error", err, "route_id", event.RouteID)
	}

	var countries []string
	if routeData != nil && routeData.Country != "" {
		countries = []string{routeData.Country}
		s.log.Trace("completeEvent: страна определена из маршрута", "event_id", eventID, "country", routeData.Country)
	} else {
		s.log.Warn("completeEvent: не удалось определить страну маршрута", "event_id", eventID, "route_id", event.RouteID)
		countries = []string{}
	}

	// Публикуем событие
	s.log.Trace("completeEvent: публикация события в Kafka", "event_id", eventID)
	if err := s.producer.PublishEventCompleted(ctx, eventID, event.RouteID, event.OwnerID, participantIDs, countries); err != nil {
		s.log.Error("completeEvent: ошибка публикации события", "error", err, "event_id", eventID)
	} else {
		s.log.Trace("completeEvent: событие опубликовано", "event_id", eventID, "countries", countries)
	}

	s.log.Info("completeEvent: поездка завершена", "event_id", eventID)
	s.log.Debug("completeEvent: завершено", "event_id", eventID)
	return nil
}

// calculateWaypointDates рассчитывает даты для точек маршрута
func (s *EventService) calculateWaypointDates(waypoints []models.RouteWaypoint, startDate time.Time, eventID uuid.UUID) []models.Waypoint {
	s.log.Trace("calculateWaypointDates: начало", "waypoints_count", len(waypoints), "start_date", startDate, "event_id", eventID)

	result := make([]models.Waypoint, 0, len(waypoints))
	currentDate := startDate

	for i, wp := range waypoints {
		plannedStart := currentDate
		plannedEnd := currentDate.Add(time.Duration(wp.DurationDays*24+wp.DurationHours) * time.Hour)
		s.log.Trace("calculateWaypointDates: точка рассчитана", "index", i, "city", wp.City, "planned_start", plannedStart, "planned_end", plannedEnd)

		result = append(result, models.Waypoint{
			ID:              uuid.New(),
			EventID:         eventID,
			RouteWaypointID: &wp.ID,
			City:            wp.City,
			Description:     wp.Description,
			Latitude:        wp.Latitude,
			Longitude:       wp.Longitude,
			PlannedStart:    &plannedStart,
			PlannedEnd:      &plannedEnd,
			Status:          "pending",
			OrderIndex:      wp.OrderIndex,
			CreatedAt:       time.Now(),
		})

		currentDate = plannedEnd
	}

	s.log.Trace("calculateWaypointDates: завершено", "result_count", len(result))
	return result
}

// recalculateWaypointDates пересчитывает даты для существующих точек
func (s *EventService) recalculateWaypointDates(waypoints []models.Waypoint, startDate time.Time) []models.Waypoint {
	s.log.Trace("recalculateWaypointDates: начало", "waypoints_count", len(waypoints), "new_start_date", startDate)

	result := make([]models.Waypoint, len(waypoints))
	currentDate := startDate

	for i, wp := range waypoints {
		durationDays := wp.PlannedEnd.Sub(*wp.PlannedStart).Hours() / 24
		durationHours := int(wp.PlannedEnd.Sub(*wp.PlannedStart).Hours()) % 24
		s.log.Trace("recalculateWaypointDates: точка", "index", i, "city", wp.City, "duration_days", durationDays, "duration_hours", durationHours)

		plannedStart := currentDate
		plannedEnd := currentDate.Add(time.Duration(int(durationDays)*24+durationHours) * time.Hour)

		result[i] = wp
		result[i].PlannedStart = &plannedStart
		result[i].PlannedEnd = &plannedEnd

		currentDate = plannedEnd
	}

	s.log.Trace("recalculateWaypointDates: завершено", "new_end_date", currentDate)
	return result
}

// activateEvents активирует planned поездки, у которых start_date <= now()
func (s *EventService) activateEvents(ctx context.Context) error {
	now := time.Now()
	s.log.Trace("activateEvents: начало", "now", now)

	plannedEvents, err := s.eventRepo.GetPlannedEventsForActivation(ctx, now)
	if err != nil {
		s.log.Error("activateEvents: ошибка получения planned событий", "error", err)
		return err
	}
	s.log.Trace("activateEvents: найдено planned событий", "count", len(plannedEvents))

	for _, eventID := range plannedEvents {
		s.log.Trace("activateEvents: активация события", "event_id", eventID)
		if err := s.eventRepo.UpdateStatus(ctx, eventID, "active"); err != nil {
			s.log.Error("activateEvents: ошибка активации", "error", err, "event_id", eventID)
		} else {
			s.log.Trace("activateEvents: событие активировано", "event_id", eventID)
		}
	}

	return nil
}

// updateEventStatuses обновляет статусы поездок (активация и завершение)
func (s *EventService) updateEventStatuses(ctx context.Context) {
	now := time.Now()
	s.log.Trace("updateEventStatuses: начало", "now", now)

	// 1. Активируем planned поездки
	s.log.Trace("updateEventStatuses: этап 1 - активация planned поездок")
	if err := s.activateEvents(ctx); err != nil {
		s.log.Error("updateEventStatuses: ошибка активации поездок", "error", err)
	}

	// 2. Завершаем active поездки, у которых end_date < now()
	s.log.Trace("updateEventStatuses: этап 2 - завершение active поездок")
	activeEvents, err := s.eventRepo.GetActiveEvents(ctx, now)
	if err != nil {
		s.log.Error("updateEventStatuses: ошибка получения активных поездок", "error", err)
		return
	}
	s.log.Trace("updateEventStatuses: найдено active поездок для завершения", "count", len(activeEvents))

	for _, eventID := range activeEvents {
		s.log.Trace("updateEventStatuses: завершение события", "event_id", eventID)
		if err := s.completeEvent(ctx, eventID); err != nil {
			s.log.Error("updateEventStatuses: ошибка завершения поездки", "error", err, "event_id", eventID)
		} else {
			s.log.Trace("updateEventStatuses: событие завершено", "event_id", eventID)
		}
	}

	s.log.Trace("updateEventStatuses: завершено")
}
