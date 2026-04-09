package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/client/auth"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/client/route"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/kafka"
	eventdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/event"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrEventNotFound          = errors.New("event not found")
	ErrNotOwner               = errors.New("user is not the owner of this event")
	ErrNotParticipant         = errors.New("user is not a participant of this event")
	ErrAlreadyParticipant     = errors.New("user is already a participant")
	ErrEventNotActive         = errors.New("event is not active")
	ErrEventAlreadyEnded      = errors.New("event has already ended")
	ErrInvalidStartDate       = errors.New("start date cannot be in the past")
	ErrInvalidWaypoint        = errors.New("invalid waypoint")
	ErrInviteNotFound         = errors.New("invitation not found")
	ErrInviteAlreadyProcessed = errors.New("invitation already processed")
	ErrInviteLinkNotFound     = errors.New("invite link not found or expired") // NEW
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
	redisClient     *redis.Client
	cfg             *config.Config
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
	redisClient *redis.Client,
	log *logger.Logger,
	cfg *config.Config,
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
		redisClient:     redisClient,
		log:             log,
		cfg:             cfg,
	}
}

// GetEventByID получает поездку по ID
func (s *EventService) GetEventByID(ctx context.Context, eventID uuid.UUID, userID *uuid.UUID) (*models.EventWithDetails, error) {
	s.log.Debug("GetEventByID: начало", "event_id", eventID, "user_id", userID)

	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, ErrEventNotFound
	}

	// Проверяем права доступа
	if !event.IsPublic {
		// Если пользователь не авторизован — доступ запрещен
		if userID == nil {
			s.log.Debug("GetEventByID: доступ запрещен, пользователь не авторизован", "event_id", eventID)
			return nil, ErrEventNotFound
		}

		// Проверяем, есть ли у пользователя какая-либо связь с событием
		participant, err := s.participantRepo.GetByUserAndEvent(ctx, eventID, *userID)
		if err != nil {
			s.log.Error("GetEventByID: ошибка проверки участника", "error", err, "event_id", eventID, "user_id", userID)
			return nil, err
		}

		// Нет никакой записи — доступ запрещен
		if participant == nil {
			s.log.Debug("GetEventByID: доступ запрещен, пользователь не связан с событием", "event_id", eventID, "user_id", userID)
			return nil, ErrEventNotFound
		}

		// Запись есть — доступ разрешен (активный участник, приглашение или заявка)
		s.log.Debug("GetEventByID: доступ разрешен", "event_id", eventID, "user_id", userID,
			"approved_by_owner", participant.ApprovedByOwner,
			"approved_by_user", participant.ApprovedByUser)
	}

	// Получаем участников
	participants, err := s.participantRepo.GetByEventID(ctx, eventID)
	if err != nil {
		s.log.Error("GetEventByID: ошибка получения участников", "error", err)
		participants = []models.Participant{}
	}

	// Получаем точки
	waypoints, err := s.waypointRepo.GetByEventID(ctx, eventID)
	if err != nil {
		s.log.Error("GetEventByID: ошибка получения точек", "error", err)
		waypoints = []models.Waypoint{}
	}

	return &models.EventWithDetails{
		Event:        *event,
		Participants: participants,
		Waypoints:    waypoints,
	}, nil
}

// GetMyEvents получает поездки пользователя (где он полноценный участник)
func (s *EventService) GetMyEvents(ctx context.Context, userID uuid.UUID, status string, page, size int) ([]models.Event, int64, error) {
	s.log.Debug("GetMyEvents: начало", "user_id", userID, "status", status, "page", page, "size", size)

	offset := (page - 1) * size
	return s.eventRepo.ListByUserID(ctx, userID, status, size, offset)
}

// GetPublicEvents получает публичные завершённые поездки
func (s *EventService) GetPublicEvents(ctx context.Context, page, size int) ([]models.Event, int64, error) {
	s.log.Debug("GetPublicEvents: начало", "page", page, "size", size)

	offset := (page - 1) * size
	return s.eventRepo.ListPublic(ctx, size, offset)
}

// UpdateEvent обновляет поездку
func (s *EventService) UpdateEvent(ctx context.Context, eventID, userID uuid.UUID, req *models.UpdateEventRequest) (*models.Event, error) {
	s.log.Debug("UpdateEvent: начало", "event_id", eventID, "user_id", userID)

	// Проверяем права
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, userID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, ErrNotOwner
	}

	// Получаем событие
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, ErrEventNotFound
	}

	// Обновляем поля
	if req.Title != nil {
		event.Title = *req.Title
	}
	if req.Description != nil {
		event.Description = *req.Description
	}
	if req.IsPublic != nil {
		event.IsPublic = *req.IsPublic
	}

	// Если меняется дата начала, нужно пересчитать точки
	if req.StartDate != nil && !req.StartDate.Equal(event.StartDate) {
		if req.StartDate.Before(time.Now()) {
			return nil, ErrInvalidStartDate
		}
		event.StartDate = *req.StartDate

		waypoints, err := s.waypointRepo.GetByEventID(ctx, eventID)
		if err != nil {
			s.log.Error("UpdateEvent: ошибка получения точек для пересчёта", "error", err)
		} else {
			newWaypoints := s.recalculateWaypointDates(waypoints, event.StartDate)
			for i, wp := range newWaypoints {
				if err := s.waypointRepo.Update(ctx, &wp); err != nil {
					s.log.Error("UpdateEvent: ошибка обновления точки", "error", err, "waypoint_id", wp.ID)
				} else {
					waypoints[i] = wp
				}
			}

			if len(waypoints) > 0 && waypoints[len(waypoints)-1].PlannedEnd != nil {
				event.EndDate = *waypoints[len(waypoints)-1].PlannedEnd
			}
		}
	}

	// Сохраняем изменения
	if err := s.eventRepo.Update(ctx, event); err != nil {
		return nil, err
	}

	// Публикуем событие
	if err := s.producer.PublishEventUpdated(ctx, eventID, userID, event.Title, event.StartDate, event.EndDate, event.IsPublic); err != nil {
		s.log.Error("UpdateEvent: ошибка публикации события", "error", err)
	}

	return event, nil
}

// DeleteEvent удаляет/отменяет поездку
func (s *EventService) DeleteEvent(ctx context.Context, eventID, userID uuid.UUID) error {
	s.log.Debug("DeleteEvent: начало", "event_id", eventID, "user_id", userID)

	// Проверяем права
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, userID)
	if err != nil {
		return err
	}
	if !isOwner {
		return ErrNotOwner
	}

	// Получаем событие
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return err
	}
	if event == nil {
		return ErrEventNotFound
	}

	// Обновляем статус
	if err := s.eventRepo.UpdateStatus(ctx, eventID, "cancelled"); err != nil {
		return err
	}

	// Публикуем событие об отмене
	if err := s.producer.PublishEventCancelled(ctx, eventID, userID); err != nil {
		s.log.Error("DeleteEvent: ошибка публикации event.cancelled", "error", err)
	}

	// Публикуем событие об удалении (для поискового индекса и других сервисов)
	if err := s.producer.PublishEventDeleted(ctx, eventID, userID); err != nil {
		s.log.Error("DeleteEvent: ошибка публикации event.deleted", "error", err)
	}

	s.log.Info("DeleteEvent: поездка отменена и помечена как удалённая", "event_id", eventID)
	return nil
}

// LeaveEvent выход из поездки
func (s *EventService) LeaveEvent(ctx context.Context, eventID, userID uuid.UUID) error {
	s.log.Debug("LeaveEvent: начало", "event_id", eventID, "user_id", userID)

	// Проверяем, не владелец ли
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, userID)
	if err != nil {
		return err
	}
	if isOwner {
		return errors.New("owner cannot leave the event")
	}

	return s.participantRepo.Remove(ctx, eventID, userID)
}

// AcceptParticipant владелец принимает участника (утверждает заявку)
func (s *EventService) AcceptParticipant(ctx context.Context, eventID, ownerID, participantID uuid.UUID) error {
	s.log.Debug("AcceptParticipant: начало", "event_id", eventID, "owner_id", ownerID, "participant_id", participantID)

	// Проверяем права
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, ownerID)
	if err != nil {
		return err
	}
	if !isOwner {
		return ErrNotOwner
	}

	// Получаем участника
	participant, err := s.participantRepo.GetByUserAndEvent(ctx, eventID, participantID)
	if err != nil {
		return err
	}
	if participant == nil {
		return errors.New("participant not found")
	}

	// Проверяем, что это заявка (approved_by_owner = false, approved_by_user = true)
	if participant.ApprovedByUser == nil {
		return errors.New("this is an invitation, not a join request")
	}
	if participant.ApprovedByOwner {
		return errors.New("participant already approved")
	}
	if !*participant.ApprovedByUser {
		return errors.New("participant has not confirmed yet")
	}

	// Обновляем approved_by_owner = true, approved_by_user оставляем как есть (nil)
	approvedTrue := true
	if err := s.participantRepo.UpdateApproval(ctx, participant.InviteID, &approvedTrue, nil); err != nil {
		return err
	}

	// Публикуем событие о присоединении участника
	go func() {
		if err := s.producer.PublishParticipantJoined(context.Background(), eventID, participantID); err != nil {
			s.log.Error("AcceptParticipant: ошибка публикации события", "error", err, "event_id", eventID, "user_id", participantID)
		}
	}()

	s.log.Info("AcceptParticipant: участник принят", "event_id", eventID, "user_id", participantID)
	return nil
}

// DeclineParticipant владелец отклоняет участника
func (s *EventService) DeclineParticipant(ctx context.Context, eventID, ownerID, participantID uuid.UUID) error {
	s.log.Debug("DeclineParticipant: начало", "event_id", eventID, "owner_id", ownerID, "participant_id", participantID)

	// Проверяем права
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, ownerID)
	if err != nil {
		return err
	}
	if !isOwner {
		return ErrNotOwner
	}

	return s.participantRepo.Remove(ctx, eventID, participantID)
}

// GetParticipants получает список участников
func (s *EventService) GetParticipants(ctx context.Context, eventID uuid.UUID) ([]models.Participant, error) {
	s.log.Debug("GetParticipants: начало", "event_id", eventID)
	return s.participantRepo.GetByEventID(ctx, eventID)
}

// GetWaypoints получает точки маршрута
func (s *EventService) GetWaypoints(ctx context.Context, eventID uuid.UUID) ([]models.Waypoint, error) {
	s.log.Debug("GetWaypoints: начало", "event_id", eventID)
	return s.waypointRepo.GetByEventID(ctx, eventID)
}

// VisitWaypoint отмечает точку посещённой
func (s *EventService) VisitWaypoint(ctx context.Context, eventID, waypointID, userID uuid.UUID, req *models.VisitWaypointRequest) error {
	s.log.Debug("VisitWaypoint: начало", "event_id", eventID, "waypoint_id", waypointID, "user_id", userID)

	// Проверяем, что пользователь участник
	isParticipant, err := s.participantRepo.IsParticipant(ctx, eventID, userID)
	if err != nil {
		return err
	}
	if !isParticipant {
		return ErrNotParticipant
	}

	// Получаем точку
	waypoint, err := s.waypointRepo.GetByID(ctx, waypointID)
	if err != nil {
		return err
	}
	if waypoint == nil {
		return ErrInvalidWaypoint
	}

	// Обновляем статус
	if err := s.waypointRepo.UpdateVisit(ctx, waypointID, req.ActualStart, req.ActualEnd); err != nil {
		return err
	}

	// Публикуем событие
	if err := s.producer.PublishWaypointVisited(ctx, eventID, waypointID, userID, waypoint.City); err != nil {
		s.log.Error("VisitWaypoint: ошибка публикации события", "error", err)
	}

	return nil
}

// AddMedia добавляет медиа к точке
func (s *EventService) AddMedia(ctx context.Context, eventID, waypointID, userID uuid.UUID, req *models.AddMediaRequest) (*models.Media, error) {
	s.log.Debug("AddMedia: начало", "event_id", eventID, "waypoint_id", waypointID, "user_id", userID)

	// Проверяем, что пользователь участник
	isParticipant, err := s.participantRepo.IsParticipant(ctx, eventID, userID)
	if err != nil {
		return nil, err
	}
	if !isParticipant {
		return nil, ErrNotParticipant
	}

	media := &models.Media{
		ID:           uuid.New(),
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
		return nil, err
	}

	return media, nil
}

// GetMedia получает медиа точки
func (s *EventService) GetMedia(ctx context.Context, waypointID uuid.UUID, page, size int) ([]models.Media, int64, error) {
	s.log.Debug("GetMedia: начало", "waypoint_id", waypointID, "page", page, "size", size)

	offset := (page - 1) * size
	return s.mediaRepo.GetByWaypointID(ctx, waypointID, size, offset)
}

// DeleteMedia удаляет медиа
func (s *EventService) DeleteMedia(ctx context.Context, mediaID, userID uuid.UUID) error {
	s.log.Debug("DeleteMedia: начало", "media_id", mediaID, "user_id", userID)

	// Проверяем права
	isOwner, err := s.mediaRepo.IsOwner(ctx, mediaID, userID)
	if err != nil {
		return err
	}
	if !isOwner {
		media, err := s.mediaRepo.GetByID(ctx, mediaID)
		if err != nil {
			return err
		}
		if media != nil {
			isEventOwner, err := s.participantRepo.IsOwner(ctx, media.EventID, userID)
			if err != nil {
				return err
			}
			if !isEventOwner {
				return errors.New("not authorized to delete this media")
			}
		}
	}

	return s.mediaRepo.Delete(ctx, mediaID)
}

// AddComment добавляет комментарий
func (s *EventService) AddComment(ctx context.Context, eventID, waypointID, userID uuid.UUID, text string) (*models.Comment, error) {
	s.log.Debug("AddComment: начало", "event_id", eventID, "waypoint_id", waypointID, "user_id", userID)

	// Проверяем, что пользователь участник
	isParticipant, err := s.participantRepo.IsParticipant(ctx, eventID, userID)
	if err != nil {
		return nil, err
	}
	if !isParticipant {
		return nil, ErrNotParticipant
	}

	comment := &models.Comment{
		ID:         uuid.New(),
		EventID:    eventID,
		WaypointID: waypointID,
		UserID:     userID,
		Text:       text,
		CreatedAt:  time.Now(),
	}

	if err := s.commentRepo.Create(ctx, comment); err != nil {
		return nil, err
	}

	return comment, nil
}

// UpdateComment обновляет комментарий
func (s *EventService) UpdateComment(ctx context.Context, commentID, userID uuid.UUID, text string) error {
	s.log.Debug("UpdateComment: начало", "comment_id", commentID, "user_id", userID)

	isAuthor, err := s.commentRepo.IsAuthor(ctx, commentID, userID)
	if err != nil {
		return err
	}
	if !isAuthor {
		return errors.New("not authorized to update this comment")
	}

	comment := &models.Comment{
		ID:   commentID,
		Text: text,
	}

	return s.commentRepo.Update(ctx, comment)
}

// DeleteComment удаляет комментарий
func (s *EventService) DeleteComment(ctx context.Context, commentID, userID uuid.UUID) error {
	s.log.Debug("DeleteComment: начало", "comment_id", commentID, "user_id", userID)

	isAuthor, err := s.commentRepo.IsAuthor(ctx, commentID, userID)
	if err != nil {
		return err
	}
	if !isAuthor {
		comment, err := s.commentRepo.GetByID(ctx, commentID)
		if err != nil {
			return err
		}
		if comment != nil {
			isEventOwner, err := s.participantRepo.IsOwner(ctx, comment.EventID, userID)
			if err != nil {
				return err
			}
			if !isEventOwner {
				return errors.New("not authorized to delete this comment")
			}
		}
	}

	return s.commentRepo.Delete(ctx, commentID)
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
			s.updateEventStatuses(ctx)
		}
	}
}

func (s *EventService) completeEvent(ctx context.Context, eventID uuid.UUID) error {
	s.log.Trace("completeEvent: начало", "event_id", eventID)

	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return err
	}
	if event == nil {
		return nil
	}

	// Обновляем статус
	if err := s.eventRepo.UpdateStatus(ctx, eventID, "completed"); err != nil {
		return err
	}

	// Получаем участников
	participants, err := s.participantRepo.GetByEventID(ctx, eventID)
	if err != nil {
		s.log.Error("completeEvent: ошибка получения участников", "error", err)
	}

	participantIDs := make([]uuid.UUID, 0, len(participants))
	for _, p := range participants {
		if p.IsActive() {
			participantIDs = append(participantIDs, p.UserID)
		}
	}

	// MVP0: получаем страну из маршрута
	routeData, err := s.routeClient.GetRouteWithWaypoints(ctx, event.RouteID.String())
	if err != nil {
		s.log.Error("completeEvent: ошибка получения маршрута", "error", err)
	}

	var countries []string
	if routeData != nil && routeData.Country != "" {
		countries = []string{routeData.Country}
	} else {
		countries = []string{}
	}

	// Публикуем событие
	if err := s.producer.PublishEventCompleted(ctx, eventID, event.RouteID, event.OwnerID, participantIDs, countries); err != nil {
		s.log.Error("completeEvent: ошибка публикации события", "error", err)
	}

	s.log.Info("completeEvent: поездка завершена", "event_id", eventID)
	return nil
}

// calculateWaypointDates рассчитывает даты для точек маршрута
func (s *EventService) calculateWaypointDates(waypoints []models.RouteWaypoint, startDate time.Time, eventID uuid.UUID) []models.Waypoint {
	result := make([]models.Waypoint, 0, len(waypoints))
	currentDate := startDate

	for _, wp := range waypoints {
		plannedStart := currentDate
		plannedEnd := currentDate.Add(time.Duration(wp.DurationDays*24+wp.DurationHours) * time.Hour)

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

	return result
}

// recalculateWaypointDates пересчитывает даты для существующих точек
func (s *EventService) recalculateWaypointDates(waypoints []models.Waypoint, startDate time.Time) []models.Waypoint {
	result := make([]models.Waypoint, len(waypoints))
	currentDate := startDate

	for i, wp := range waypoints {
		durationDays := wp.PlannedEnd.Sub(*wp.PlannedStart).Hours() / 24
		durationHours := int(wp.PlannedEnd.Sub(*wp.PlannedStart).Hours()) % 24

		plannedStart := currentDate
		plannedEnd := currentDate.Add(time.Duration(int(durationDays)*24+durationHours) * time.Hour)

		result[i] = wp
		result[i].PlannedStart = &plannedStart
		result[i].PlannedEnd = &plannedEnd

		currentDate = plannedEnd
	}

	return result
}

// activateEvents активирует planned поездки, у которых start_date <= now()
func (s *EventService) activateEvents(ctx context.Context) error {
	plannedEvents, err := s.eventRepo.GetPlannedEventsForActivation(ctx, time.Now())
	if err != nil {
		return err
	}

	for _, eventID := range plannedEvents {
		s.log.Debug("Активация поездки", "event_id", eventID)
		if err := s.eventRepo.UpdateStatus(ctx, eventID, "active"); err != nil {
			s.log.Error("Ошибка активации поездки", "error", err, "event_id", eventID)
		}
	}

	return nil
}

// updateEventStatuses обновляет статусы поездок (активация и завершение)
func (s *EventService) updateEventStatuses(ctx context.Context) {
	now := time.Now()

	// 1. Активируем planned поездки
	if err := s.activateEvents(ctx); err != nil {
		s.log.Error("Ошибка активации поездок", "error", err)
	}

	// 2. Завершаем active поездки, у которых end_date < now()
	activeEvents, err := s.eventRepo.GetActiveEvents(ctx, now)
	if err != nil {
		s.log.Error("Ошибка получения активных поездок", "error", err)
		return
	}

	for _, eventID := range activeEvents {
		s.log.Debug("Завершение поездки", "event_id", eventID)
		if err := s.completeEvent(ctx, eventID); err != nil {
			s.log.Error("Ошибка завершения поездки", "error", err, "event_id", eventID)
		}
	}
}

// GetUserEmail получает email пользователя из Auth Service
func (s *EventService) GetUserEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.authClient.GetUserEmail(ctx, userID)
}

// CreateEvent создаёт новую поездку
func (s *EventService) CreateEvent(ctx context.Context, userID uuid.UUID, req *models.CreateEventRequest) (*models.EventWithDetails, error) {
	s.log.Debug("CreateEvent: начало", "user_id", userID, "route_id", req.RouteID, "title", req.Title, "start_date", req.StartDate)

	// Проверяем дату начала
	if req.StartDate.Before(time.Now()) {
		s.log.Debug("CreateEvent: дата начала в прошлом", "start_date", req.StartDate)
		return nil, ErrInvalidStartDate
	}

	// Получаем маршрут из Route Service
	routeData, err := s.routeClient.GetRouteWithWaypoints(ctx, req.RouteID.String())
	if err != nil {
		s.log.Error("CreateEvent: ошибка получения маршрута", "error", err, "route_id", req.RouteID)
		return nil, fmt.Errorf("failed to get route: %w", err)
	}

	// Создаём ID события
	eventID := uuid.New()
	now := time.Now()

	// Рассчитываем даты точек
	waypoints := s.calculateWaypointDates(routeData.Waypoints, req.StartDate, eventID)

	// Вычисляем дату окончания
	endDate := req.StartDate
	if len(waypoints) > 0 && waypoints[len(waypoints)-1].PlannedEnd != nil {
		endDate = *waypoints[len(waypoints)-1].PlannedEnd
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

	// Добавляем владельца: approved_by_owner = true, approved_by_user = true
	approvedTrue := true
	participant := &models.Participant{
		InviteID:        uuid.New(),
		EventID:         eventID,
		UserID:          userID,
		Role:            "owner",
		ApprovedByOwner: true,
		ApprovedByUser:  &approvedTrue,
	}
	if err := s.participantRepo.Add(ctx, participant); err != nil {
		s.log.Error("CreateEvent: ошибка добавления владельца", "error", err, "event_id", eventID)
	}

	// Обрабатываем приглашения
	participants := []uuid.UUID{userID}
	if req.IsGroup && len(req.InvitedEmails) > 0 {
		for _, email := range req.InvitedEmails {
			// Проверяем, существует ли пользователь
			userInfo, err := s.authClient.GetUserByEmail(ctx, email)
			if err != nil {
				s.log.Error("CreateEvent: ошибка проверки пользователя", "error", err, "email", email)
				continue
			}

			var invite *models.Participant

			if userInfo != nil {
				// Пользователь уже зарегистрирован
				invite = &models.Participant{
					InviteID:        uuid.New(),
					EventID:         eventID,
					UserID:          userInfo.UserID,
					Role:            "member",
					ApprovedByOwner: true,
					ApprovedByUser:  nil, // null = приглашение
					InvitedEmail:    "",
				}
				participants = append(participants, userInfo.UserID)
				s.log.Trace("CreateEvent: пользователь найден, создаём приглашение", "event_id", eventID, "user_id", userInfo.UserID, "email", email)
			} else {
				// Пользователь не зарегистрирован
				invite = &models.Participant{
					InviteID:        uuid.New(),
					EventID:         eventID,
					UserID:          uuid.Nil,
					Role:            "member",
					ApprovedByOwner: true,
					ApprovedByUser:  nil, // null = приглашение
					InvitedEmail:    email,
				}
				s.log.Trace("CreateEvent: пользователь не найден, создаём приглашение", "event_id", eventID, "email", email)
			}

			if err := s.participantRepo.Add(ctx, invite); err != nil {
				s.log.Error("CreateEvent: ошибка создания", "error", err, "email", email)
			}
		}
	}

	// Публикуем событие
	if err := s.producer.PublishEventCreated(ctx, eventID, req.RouteID, userID, req.Title, req.StartDate, endDate, participants); err != nil {
		s.log.Error("CreateEvent: ошибка публикации события", "error", err, "event_id", eventID)
	}

	s.log.Info("CreateEvent: поездка создана", "event_id", eventID, "user_id", userID)

	return &models.EventWithDetails{
		Event:        *event,
		Participants: []models.Participant{*participant},
		Waypoints:    waypoints,
	}, nil
}

// JoinEvent запрос на участие в поездке (пользователь подаёт заявку)
func (s *EventService) JoinEvent(ctx context.Context, eventID, userID uuid.UUID) error {
	s.log.Debug("JoinEvent: начало", "event_id", eventID, "user_id", userID)

	// Проверяем существование события
	exists, err := s.eventRepo.Exists(ctx, eventID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrEventNotFound
	}

	// Проверяем, не участник ли уже
	isParticipant, err := s.participantRepo.IsParticipant(ctx, eventID, userID)
	if err != nil {
		return err
	}
	if isParticipant {
		return ErrAlreadyParticipant
	}

	// Проверяем, есть ли уже запись (например, приглашение)
	existing, err := s.participantRepo.GetByUserAndEvent(ctx, eventID, userID)
	if err != nil {
		return err
	}
	if existing != nil {
		// Если уже есть приглашение (approved_by_user == nil, approved_by_owner == true)
		if existing.ApprovedByUser == nil && existing.ApprovedByOwner {
			// Пользователь получил приглашение и теперь принимает его
			// Нужно: approved_by_owner оставить true, approved_by_user поставить true
			approvedTrue := true
			return s.participantRepo.UpdateApproval(ctx, existing.InviteID, nil, &approvedTrue)
		}

		// Если уже есть заявка (approved_by_owner == false, approved_by_user == true)
		if existing.ApprovedByUser != nil && *existing.ApprovedByUser && !existing.ApprovedByOwner {
			// Пользователь уже подал заявку, ничего не делаем
			s.log.Debug("JoinEvent: пользователь уже подал заявку", "event_id", eventID, "user_id", userID)
			return nil
		}

		// Если запись в другом статусе — ошибка
		return errors.New("participant record exists in unexpected state")
	}

	// Добавляем участника с approved_by_owner = false, approved_by_user = true
	approvedTrue := true
	participant := &models.Participant{
		InviteID:        uuid.New(),
		EventID:         eventID,
		UserID:          userID,
		Role:            "member",
		ApprovedByOwner: false,
		ApprovedByUser:  &approvedTrue,
	}
	return s.participantRepo.Add(ctx, participant)
}

// AcceptInvite пользователь принимает приглашение по invite_id
func (s *EventService) AcceptInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	s.log.Debug("AcceptInvite: начало", "invite_id", inviteID, "user_id", userID)

	// Получаем приглашение по invite_id
	participant, err := s.participantRepo.GetByInviteID(ctx, inviteID)
	if err != nil {
		return err
	}
	if participant == nil {
		return ErrInviteNotFound
	}

	// Проверяем, что приглашение предназначено этому пользователю
	if participant.UserID != userID {
		s.log.Warn("AcceptInvite: приглашение не принадлежит пользователю",
			"invite_id", inviteID,
			"expected_user", participant.UserID,
			"actual_user", userID)
		return ErrInviteNotFound
	}

	// Проверяем, что владелец подтвердил
	if !participant.ApprovedByOwner {
		return errors.New("invitation not approved by owner")
	}

	// Проверяем, что это приглашение (approved_by_user == nil)
	if participant.ApprovedByUser != nil {
		return ErrInviteAlreadyProcessed
	}

	// Обновляем approved_by_user = true
	approvedTrue := true
	return s.participantRepo.UpdateApproval(ctx, inviteID, nil, &approvedTrue)
}

// DeclineInvite пользователь отклоняет приглашение по invite_id
func (s *EventService) DeclineInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	s.log.Debug("DeclineInvite: начало", "invite_id", inviteID, "user_id", userID)

	// Получаем приглашение по invite_id
	participant, err := s.participantRepo.GetByInviteID(ctx, inviteID)
	if err != nil {
		return err
	}
	if participant == nil {
		return ErrInviteNotFound
	}

	// Проверяем, что приглашение предназначено этому пользователю
	if participant.UserID != userID {
		s.log.Warn("DeclineInvite: приглашение не принадлежит пользователю",
			"invite_id", inviteID,
			"expected_user", participant.UserID,
			"actual_user", userID)
		return ErrInviteNotFound
	}

	// Проверяем, что это приглашение (approved_by_user == nil)
	if participant.ApprovedByUser != nil {
		return ErrInviteAlreadyProcessed
	}

	// Удаляем запись (отклонение)
	return s.participantRepo.Remove(ctx, participant.EventID, userID)
}

// InviteParticipants владелец приглашает новых участников после создания поездки
func (s *EventService) InviteParticipants(ctx context.Context, eventID, ownerID uuid.UUID, emails []string) error {
	s.log.Debug("InviteParticipants: начало", "event_id", eventID, "owner_id", ownerID, "emails", emails)

	// Проверяем права
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, ownerID)
	if err != nil {
		return err
	}
	if !isOwner {
		return ErrNotOwner
	}

	// Проверяем, что поездка ещё не началась
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return err
	}
	if event == nil {
		return ErrEventNotFound
	}
	if event.StartDate.Before(time.Now()) {
		return errors.New("cannot invite to event that has already started")
	}

	for _, email := range emails {
		// Проверяем, не пытается ли владелец пригласить себя
		userInfo, err := s.authClient.GetUserByEmail(ctx, email)
		if err != nil {
			s.log.Error("InviteParticipants: ошибка проверки пользователя", "error", err, "email", email)
			continue
		}

		if userInfo != nil && userInfo.UserID == ownerID {
			s.log.Debug("InviteParticipants: владелец пытается пригласить себя, пропускаем", "email", email)
			continue
		}

		var participant *models.Participant

		if userInfo != nil {
			// Пользователь уже зарегистрирован
			participant = &models.Participant{
				InviteID:        uuid.New(),
				EventID:         eventID,
				UserID:          userInfo.UserID,
				Role:            "member",
				ApprovedByOwner: true,
				ApprovedByUser:  nil, // null = приглашение
			}
		} else {
			// Пользователь не зарегистрирован
			participant = &models.Participant{
				InviteID:        uuid.New(),
				EventID:         eventID,
				UserID:          uuid.Nil,
				Role:            "member",
				ApprovedByOwner: true,
				ApprovedByUser:  nil, // null = приглашение
				InvitedEmail:    email,
			}
		}

		if err := s.participantRepo.Add(ctx, participant); err != nil {
			s.log.Error("InviteParticipants: ошибка создания", "error", err, "email", email)
		} else {
			s.log.Info("InviteParticipants: приглашение создано", "event_id", eventID, "email", email)
		}
	}

	return nil
}

// GetInvitesByUserID получает приглашения пользователя (approved_by_owner = true AND approved_by_user IS NULL)
func (s *EventService) GetInvitesByUserID(ctx context.Context, userID uuid.UUID) ([]models.Invite, error) {
	s.log.Debug("GetInvitesByUserID: начало", "user_id", userID)
	return s.participantRepo.GetInvitesByUserID(ctx, userID)
}

// EventParticipantsStats статистика участников события
type EventParticipantsStats struct {
	TotalInvites      int
	TotalParticipants int
}

// GetParticipantsStats возвращает статистику участников для списка событий
func (s *EventService) GetParticipantsStats(ctx context.Context, eventIDs []uuid.UUID) (map[uuid.UUID]eventdto.EventParticipantsStatsDTO, error) {
	s.log.Debug("GetParticipantsStats: начало", "count", len(eventIDs))

	stats, err := s.eventRepo.GetParticipantsStats(ctx, eventIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]eventdto.EventParticipantsStatsDTO, len(stats))
	for id, stat := range stats {
		result[id] = eventdto.EventParticipantsStatsDTO{
			TotalInvites:      stat.TotalInvites,
			TotalParticipants: stat.TotalParticipants,
		}
	}

	return result, nil
}

// IsEventOwner проверяет, является ли пользователь владельцем события
func (s *EventService) IsEventOwner(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	return s.participantRepo.IsOwner(ctx, eventID, userID)
}

// IsEventParticipant проверяет, является ли пользователь активным участником события
func (s *EventService) IsEventParticipant(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	return s.participantRepo.IsParticipant(ctx, eventID, userID)
}

// CreateInviteLink создаёт временную ссылку-приглашение
func (s *EventService) CreateInviteLink(ctx context.Context, eventID, userID uuid.UUID, ttl time.Duration) (*eventdto.InviteLinkResponse, error) {
	s.log.Debug("CreateInviteLink: начало", "event_id", eventID, "user_id", userID, "ttl", ttl)

	// Проверяем, что пользователь имеет право создавать приглашения (только владелец)
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, userID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		s.log.Warn("CreateInviteLink: попытка создания ссылки не владельцем", "user_id", userID, "event_id", eventID)
		return nil, ErrNotOwner
	}

	// Проверяем, что поездка существует
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, ErrEventNotFound
	}

	// Генерируем уникальный link_id
	linkID := uuid.New().String()

	// Сохраняем в Redis
	expiresAt := time.Now().Add(ttl)
	redisKey := fmt.Sprintf("invite_link:%s", linkID)
	err = s.redisClient.Set(ctx, redisKey, eventID.String(), ttl).Err()
	if err != nil {
		s.log.Error("CreateInviteLink: ошибка сохранения в Redis", "error", err, "key", redisKey)
		return nil, err
	}

	s.log.Info("CreateInviteLink: ссылка-приглашение создана", "event_id", eventID, "link_id", linkID, "expires_at", expiresAt)

	return &eventdto.InviteLinkResponse{
		LinkID:    linkID,
		ExpiresAt: expiresAt,
	}, nil
}

// GetEventByInviteLink получает информацию о поездке по ссылке-приглашению
func (s *EventService) GetEventByInviteLink(ctx context.Context, linkID string) (*eventdto.EventPreviewResponse, error) {
	s.log.Debug("GetEventByInviteLink: начало", "link_id", linkID)

	// Получаем event_id из Redis
	redisKey := fmt.Sprintf("invite_link:%s", linkID)
	eventIDStr, err := s.redisClient.Get(ctx, redisKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			s.log.Debug("GetEventByInviteLink: ссылка не найдена или истекла", "link_id", linkID)
			return nil, ErrInviteLinkNotFound
		}
		s.log.Error("GetEventByInviteLink: ошибка получения из Redis", "error", err, "key", redisKey)
		return nil, err
	}

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		return nil, ErrInviteLinkNotFound
	}

	// Получаем информацию о поездке
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, ErrEventNotFound
	}

	// Получаем количество участников
	participantsCount, err := s.participantRepo.CountByEventID(ctx, eventID)
	if err != nil {
		s.log.Error("GetEventByInviteLink: ошибка получения количества участников", "error", err)
		participantsCount = 0
	}

	// Получаем точки маршрута
	waypoints, err := s.waypointRepo.GetByEventID(ctx, eventID)
	if err != nil {
		s.log.Error("GetEventByInviteLink: ошибка получения точек", "error", err)
		return nil, err
	}

	// Формируем DTO для точек
	waypointDTOs := make([]eventdto.WaypointPreviewResponse, 0, len(waypoints))
	for _, wp := range waypoints {
		waypointDTOs = append(waypointDTOs, eventdto.WaypointPreviewResponse{
			ID:           wp.ID,
			City:         wp.City,
			Description:  wp.Description,
			PlannedStart: *wp.PlannedStart,
			PlannedEnd:   *wp.PlannedEnd,
			OrderIndex:   wp.OrderIndex,
		})
	}

	s.log.Debug("GetEventByInviteLink: поездка получена", "event_id", eventID, "link_id", linkID, "waypoints_count", len(waypointDTOs))

	return &eventdto.EventPreviewResponse{
		ID:                event.ID,
		Title:             event.Title,
		Description:       event.Description,
		StartDate:         event.StartDate,
		EndDate:           event.EndDate,
		IsGroup:           event.IsGroup,
		OwnerID:           event.OwnerID,
		ParticipantsCount: participantsCount,
		Waypoints:         waypointDTOs,
	}, nil
}

// GetInviteLinkTTL возвращает TTL для ссылок-приглашений
func (s *EventService) GetInviteLinkTTL() time.Duration {
	return s.cfg.InviteLinkTTL
}
