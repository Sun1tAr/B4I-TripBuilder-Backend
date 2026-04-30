package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/client/auth"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/client/route"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/client/user"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/kafka"
	eventdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/event"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/metrics"
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
	ErrInviteLinkNotFound     = errors.New("invite link not found or expired")
)

var eventMetrics = metrics.NewBusinessMetrics("event")

var (
	eventsCreatedTotal   = eventMetrics.NewCounter("events_created_total", "Total events created")
	eventsCompletedTotal = eventMetrics.NewCounter("events_completed_total", "Total events completed")
	activeEvents         = eventMetrics.NewGauge("active_events", "Current active events")
)

type EventService struct {
	eventRepo       repository.EventRepository
	participantRepo repository.ParticipantRepository
	waypointRepo    repository.WaypointRepository
	mediaRepo       repository.MediaRepository
	commentRepo     repository.CommentRepository
	transferRepo    repository.EventTransferRepository
	routeClient     *route.Client
	authClient      *auth.Client
	userClient      *user.Client
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
	transferRepo repository.EventTransferRepository,
	routeClient *route.Client,
	authClient *auth.Client,
	userClient *user.Client,
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
		transferRepo:    transferRepo,
		routeClient:     routeClient,
		authClient:      authClient,
		userClient:      userClient,
		producer:        producer,
		redisClient:     redisClient,
		log:             log,
		cfg:             cfg,
	}
}

// ========== ВСПОМОГАТЕЛЬНЫЕ МЕТОДЫ ==========

func (s *EventService) getUserProfile(ctx context.Context, userID uuid.UUID) (*user.ProfileResponse, error) {
	return s.userClient.GetProfile(ctx, userID)
}

func (s *EventService) getUserFullName(ctx context.Context, userID uuid.UUID) string {
	return s.userClient.GetFullName(ctx, userID)
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// ========== ПОЛУЧЕНИЕ ПОЕЗДКИ (С ПЕРЕМЕЩЕНИЯМИ) ==========

func (s *EventService) GetEventByID(ctx context.Context, eventID uuid.UUID, userID *uuid.UUID) (*models.EventWithDetails, error) {
	s.log.Debug("GetEventByID: начало", "event_id", eventID, "user_id", userID)

	event, waypoints, transfers, err := s.eventRepo.GetByIDWithDetails(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, ErrEventNotFound
	}

	if !event.IsPublic {
		if userID == nil {
			return nil, ErrEventNotFound
		}
		participant, err := s.participantRepo.GetByUserAndEvent(ctx, eventID, *userID)
		if err != nil {
			return nil, err
		}
		if participant == nil {
			return nil, ErrEventNotFound
		}
	}

	participants, err := s.participantRepo.GetByEventID(ctx, eventID)
	if err != nil {
		participants = []models.Participant{}
	}

	return &models.EventWithDetails{
		Event:        *event,
		Participants: participants,
		Waypoints:    waypoints,
		Transfers:    transfers,
	}, nil
}

// ========== СОЗДАНИЕ ПОЕЗДКИ (С ПЕРЕМЕЩЕНИЯМИ) ==========

func (s *EventService) CreateEvent(ctx context.Context, userID uuid.UUID, req *models.CreateEventRequest) (*models.EventWithDetails, error) {
	s.log.Debug("CreateEvent: начало", "user_id", userID, "route_id", req.RouteID, "title", req.Title)

	if req.StartDate.Before(time.Now()) {
		return nil, ErrInvalidStartDate
	}

	routeData, err := s.routeClient.GetRouteWithWaypointsAndTransfers(ctx, req.RouteID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get route: %w", err)
	}

	eventID := uuid.New()
	now := time.Now()

	waypoints, transfers := s.calculateWaypointAndTransferDates(routeData.Waypoints, routeData.Transfers, req.StartDate, eventID)

	endDate := req.StartDate
	if len(transfers) > 0 && transfers[len(transfers)-1].PlannedEnd != nil {
		endDate = *transfers[len(transfers)-1].PlannedEnd
	} else if len(waypoints) > 0 && waypoints[len(waypoints)-1].PlannedEnd != nil {
		endDate = *waypoints[len(waypoints)-1].PlannedEnd
	}

	coverURL := sql.NullString{String: req.CoverURL, Valid: req.CoverURL != ""}
	if !coverURL.Valid && routeData.ImageURL != "" {
		coverURL = sql.NullString{String: routeData.ImageURL, Valid: true}
	}

	event := &models.Event{
		ID:          eventID,
		RouteID:     req.RouteID,
		Title:       req.Title,
		Description: req.Description,
		CoverURL:    coverURL,
		OwnerID:     userID,
		StartDate:   req.StartDate,
		EndDate:     endDate,
		IsGroup:     req.IsGroup,
		Status:      "planned",
		IsPublic:    req.IsPublic,
		CreatedAt:   now,
	}

	if err := s.eventRepo.CreateWithWaypointsAndTransfers(ctx, event, waypoints, transfers); err != nil {
		return nil, err
	}

	approvedTrue := true
	ownerParticipant := &models.Participant{
		InviteID:        uuid.New(),
		EventID:         eventID,
		UserID:          userID,
		Role:            "owner",
		ApprovedByOwner: true,
		ApprovedByUser:  &approvedTrue,
	}
	s.participantRepo.Add(ctx, ownerParticipant)

	participants := []uuid.UUID{userID}
	invitedUsers := make([]eventdto.InvitedUser, 0)
	inviterName := s.getUserFullName(ctx, userID)

	if req.IsGroup && len(req.InvitedEmails) > 0 {
		for _, email := range req.InvitedEmails {
			if email == "" {
				continue
			}
			userInfo, err := s.authClient.GetUserByEmail(ctx, email)
			if err != nil {
				continue
			}
			if userInfo != nil && userInfo.UserID == userID {
				continue
			}

			var invite *models.Participant
			var invitedUser eventdto.InvitedUser

			if userInfo != nil {
				invite = &models.Participant{
					InviteID: uuid.New(), EventID: eventID, UserID: userInfo.UserID,
					Role: "member", ApprovedByOwner: true, ApprovedByUser: nil,
				}
				participants = append(participants, userInfo.UserID)
				invitedUser = eventdto.InvitedUser{UserID: userInfo.UserID, Email: email, FirstName: s.getUserFullName(ctx, userInfo.UserID)}
			} else {
				invite = &models.Participant{
					InviteID: uuid.New(), EventID: eventID, UserID: uuid.Nil,
					Role: "member", ApprovedByOwner: true, ApprovedByUser: nil, InvitedEmail: email,
				}
				invitedUser = eventdto.InvitedUser{UserID: uuid.Nil, Email: email, FirstName: email}
			}

			s.participantRepo.Add(ctx, invite)
			invitedUsers = append(invitedUsers, invitedUser)
		}

		if len(invitedUsers) > 0 {
			go s.producer.PublishEventInvite(context.Background(), eventID, req.Title, routeData.Title, userID, inviterName, req.StartDate, invitedUsers)
		}
	}

	go s.producer.PublishEventCreated(context.Background(), eventID, req.RouteID, userID, req.Title, req.StartDate, endDate, participants)

	allParticipants, _ := s.participantRepo.GetByEventID(ctx, eventID)

	eventsCreatedTotal.Inc()
	return &models.EventWithDetails{
		Event: *event, Participants: allParticipants, Waypoints: waypoints, Transfers: transfers,
	}, nil
}

// ========== РАСЧЕТ ДАТ С ПЕРЕМЕЩЕНИЯМИ ==========

func (s *EventService) calculateWaypointAndTransferDates(
	routeWaypoints []route.RouteWaypoint,
	routeTransfers []route.Transfer,
	startDate time.Time,
	eventID uuid.UUID,
) ([]models.Waypoint, []models.EventTransfer) {
	now := time.Now()
	waypoints := make([]models.Waypoint, 0, len(routeWaypoints))
	orderToWaypoint := make(map[int]*models.Waypoint)

	currentTime := startDate

	// 1. Создаем все точки с начальными датами (без учета трансферов)
	for _, rwp := range routeWaypoints {
		wpID := uuid.New()
		plannedStart := currentTime
		plannedEnd := currentTime.Add(time.Duration(rwp.DurationDays*24+rwp.DurationHours) * time.Hour)

		wp := models.Waypoint{
			ID:           wpID,
			EventID:      eventID,
			City:         rwp.City,
			Description:  rwp.Description,
			Latitude:     rwp.Latitude,
			Longitude:    rwp.Longitude,
			PlannedStart: &plannedStart,
			PlannedEnd:   &plannedEnd,
			Status:       "pending",
			OrderIndex:   rwp.OrderIndex,
			CreatedAt:    now,
		}
		waypoints = append(waypoints, wp)
		orderToWaypoint[rwp.OrderIndex] = &waypoints[len(waypoints)-1]
		currentTime = plannedEnd
	}

	// 2. Вставляем трансферы и корректируем даты ПОСЛЕДУЮЩИХ точек
	transfers := make([]models.EventTransfer, 0, len(routeTransfers))

	for i, rt := range routeTransfers {
		fromWP := orderToWaypoint[rt.OrderIndex]

		toOrder := rt.OrderIndex + 1
		if toOrder >= len(routeWaypoints) {
			toOrder = 0
		}
		toWP := orderToWaypoint[toOrder]

		if fromWP == nil || toWP == nil {
			continue
		}

		transferStart := *fromWP.PlannedEnd
		transferDuration := time.Duration(rt.DurationHours)*time.Hour + time.Duration(rt.DurationMinutes)*time.Minute
		transferEnd := transferStart.Add(transferDuration)

		transfers = append(transfers, models.EventTransfer{
			ID: uuid.New(), EventID: eventID,
			FromWaypointID: fromWP.ID, ToWaypointID: toWP.ID,
			DurationHours: rt.DurationHours, DurationMinutes: rt.DurationMinutes,
			TransportType: models.TransportType(rt.TransportType),
			PlannedStart:  &transferStart, PlannedEnd: &transferEnd,
			OrderIndex: i, CreatedAt: now,
		})

		// НЕ обновляем даты для первой точки при кольцевом возврате
		if toOrder == 0 {
			continue // ← ВОТ КЛЮЧЕВОЕ ИСПРАВЛЕНИЕ
		}

		// Сдвигаем начало следующей точки
		newStart := transferEnd
		toWP.PlannedStart = &newStart

		// Пересчитываем конец
		stayDuration := toWP.PlannedEnd.Sub(*fromWP.PlannedEnd)
		newEnd := newStart.Add(stayDuration)
		toWP.PlannedEnd = &newEnd
	}

	return waypoints, transfers
}

// ========== ПОСЕЩЕНИЕ ТОЧКИ (С ОБНОВЛЕНИЕМ ТРАНСФЕРА) ==========

func (s *EventService) VisitWaypoint(ctx context.Context, eventID, waypointID, userID uuid.UUID, req *models.VisitWaypointRequest) error {
	s.log.Debug("VisitWaypoint: начало", "event_id", eventID, "waypoint_id", waypointID)

	isParticipant, err := s.participantRepo.IsParticipant(ctx, eventID, userID)
	if err != nil {
		return err
	}
	if !isParticipant {
		return ErrNotParticipant
	}

	waypoint, err := s.waypointRepo.GetByID(ctx, waypointID)
	if err != nil || waypoint == nil {
		return ErrInvalidWaypoint
	}

	if err := s.waypointRepo.UpdateVisit(ctx, waypointID, req.ActualStart, req.ActualEnd); err != nil {
		return err
	}

	if req.ActualEnd != nil {
		transfers, err := s.transferRepo.GetByEventID(ctx, eventID)
		if err == nil {
			for _, t := range transfers {
				if t.FromWaypointID == waypointID {
					s.transferRepo.UpdateActualTimes(ctx, t.ID, req.ActualEnd, nil)
					break
				}
			}
		}
	}

	return s.producer.PublishWaypointVisited(ctx, eventID, waypointID, userID, waypoint.City)
}

// ========== ОСТАЛЬНЫЕ МЕТОДЫ БЕЗ ИЗМЕНЕНИЙ ==========

func (s *EventService) GetMyEvents(ctx context.Context, userID uuid.UUID, status string, page, size int) ([]models.Event, int64, error) {
	offset := (page - 1) * size
	return s.eventRepo.ListByUserID(ctx, userID, status, size, offset)
}

func (s *EventService) GetPublicEvents(ctx context.Context, page, size int) ([]models.Event, int64, error) {
	offset := (page - 1) * size
	return s.eventRepo.ListPublic(ctx, size, offset)
}

func (s *EventService) UpdateEvent(ctx context.Context, eventID, userID uuid.UUID, req *models.UpdateEventRequest) (*models.Event, error) {
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, userID)
	if err != nil || !isOwner {
		return nil, ErrNotOwner
	}
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil || event == nil {
		return nil, ErrEventNotFound
	}
	if req.Title != nil {
		event.Title = *req.Title
	}
	if req.Description != nil {
		event.Description = *req.Description
	}
	if req.CoverURL != nil {
		event.CoverURL = sql.NullString{String: *req.CoverURL, Valid: *req.CoverURL != ""}
	}
	if req.IsPublic != nil {
		event.IsPublic = *req.IsPublic
	}
	if req.StartDate != nil && !req.StartDate.Equal(event.StartDate) {
		if req.StartDate.Before(time.Now()) {
			return nil, ErrInvalidStartDate
		}
		event.StartDate = *req.StartDate
	}
	if err := s.eventRepo.Update(ctx, event); err != nil {
		return nil, err
	}
	s.producer.PublishEventUpdated(ctx, eventID, userID, event.Title, event.StartDate, event.EndDate, event.IsPublic)
	return event, nil
}

func (s *EventService) DeleteEvent(ctx context.Context, eventID, userID uuid.UUID) error {
	isOwner, err := s.participantRepo.IsOwner(ctx, eventID, userID)
	if err != nil || !isOwner {
		return ErrNotOwner
	}
	if err := s.eventRepo.UpdateStatus(ctx, eventID, "cancelled"); err != nil {
		return err
	}
	s.producer.PublishEventCancelled(ctx, eventID, userID)
	s.producer.PublishEventDeleted(ctx, eventID, userID)
	return nil
}

func (s *EventService) JoinEvent(ctx context.Context, eventID, userID uuid.UUID) error {
	exists, _ := s.eventRepo.Exists(ctx, eventID)
	if !exists {
		return ErrEventNotFound
	}
	isParticipant, _ := s.participantRepo.IsParticipant(ctx, eventID, userID)
	if isParticipant {
		return ErrAlreadyParticipant
	}
	existing, _ := s.participantRepo.GetByUserAndEvent(ctx, eventID, userID)
	if existing != nil {
		if existing.ApprovedByUser == nil && existing.ApprovedByOwner {
			approvedTrue := true
			return s.participantRepo.UpdateApproval(ctx, existing.InviteID, nil, &approvedTrue)
		}
		return nil
	}
	approvedTrue := true
	return s.participantRepo.Add(ctx, &models.Participant{
		InviteID: uuid.New(), EventID: eventID, UserID: userID,
		Role: "member", ApprovedByOwner: false, ApprovedByUser: &approvedTrue,
	})
}

func (s *EventService) LeaveEvent(ctx context.Context, eventID, userID uuid.UUID) error {
	isOwner, _ := s.participantRepo.IsOwner(ctx, eventID, userID)
	if isOwner {
		return errors.New("owner cannot leave the event")
	}
	return s.participantRepo.Remove(ctx, eventID, userID)
}

func (s *EventService) AcceptParticipant(ctx context.Context, eventID, ownerID, participantID uuid.UUID) error {
	isOwner, _ := s.participantRepo.IsOwner(ctx, eventID, ownerID)
	if !isOwner {
		return ErrNotOwner
	}
	participant, _ := s.participantRepo.GetByUserAndEvent(ctx, eventID, participantID)
	if participant == nil {
		return errors.New("participant not found")
	}
	approvedTrue := true
	if err := s.participantRepo.UpdateApproval(ctx, participant.InviteID, &approvedTrue, nil); err != nil {
		return err
	}
	go s.producer.PublishParticipantJoined(context.Background(), eventID, participantID)
	return nil
}

func (s *EventService) DeclineParticipant(ctx context.Context, eventID, ownerID, participantID uuid.UUID) error {
	isOwner, _ := s.participantRepo.IsOwner(ctx, eventID, ownerID)
	if !isOwner {
		return ErrNotOwner
	}
	return s.participantRepo.Remove(ctx, eventID, participantID)
}

func (s *EventService) AcceptInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	participant, _ := s.participantRepo.GetByInviteID(ctx, inviteID)
	if participant == nil {
		return ErrInviteNotFound
	}
	if participant.UserID != userID || !participant.ApprovedByOwner || participant.ApprovedByUser != nil {
		return ErrInviteAlreadyProcessed
	}
	approvedTrue := true
	return s.participantRepo.UpdateApproval(ctx, inviteID, nil, &approvedTrue)
}

func (s *EventService) DeclineInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	participant, _ := s.participantRepo.GetByInviteID(ctx, inviteID)
	if participant == nil || participant.UserID != userID || participant.ApprovedByUser != nil {
		return ErrInviteAlreadyProcessed
	}
	return s.participantRepo.Remove(ctx, participant.EventID, userID)
}

func (s *EventService) InviteParticipants(ctx context.Context, eventID, ownerID uuid.UUID, emails []string) error {
	isOwner, _ := s.participantRepo.IsOwner(ctx, eventID, ownerID)
	if !isOwner {
		return ErrNotOwner
	}
	event, _ := s.eventRepo.GetByID(ctx, eventID)
	if event == nil {
		return ErrEventNotFound
	}
	routeData, _ := s.routeClient.GetRouteWithWaypoints(ctx, event.RouteID.String())
	if routeData == nil {
		routeData = &models.RouteWithWaypoints{Title: ""}
	}
	inviterName := s.getUserFullName(ctx, ownerID)
	invitedUsers := make([]eventdto.InvitedUser, 0)

	for _, email := range emails {
		userInfo, _ := s.authClient.GetUserByEmail(ctx, email)
		if userInfo != nil && userInfo.UserID == ownerID {
			continue
		}
		var participant *models.Participant
		var invitedUser eventdto.InvitedUser
		if userInfo != nil {
			participant = &models.Participant{InviteID: uuid.New(), EventID: eventID, UserID: userInfo.UserID, Role: "member", ApprovedByOwner: true, ApprovedByUser: nil}
			invitedUser = eventdto.InvitedUser{UserID: userInfo.UserID, Email: email, FirstName: s.getUserFullName(ctx, userInfo.UserID)}
		} else {
			participant = &models.Participant{InviteID: uuid.New(), EventID: eventID, UserID: uuid.Nil, Role: "member", ApprovedByOwner: true, ApprovedByUser: nil, InvitedEmail: email}
			invitedUser = eventdto.InvitedUser{UserID: uuid.Nil, Email: email, FirstName: email}
		}
		s.participantRepo.Add(ctx, participant)
		invitedUsers = append(invitedUsers, invitedUser)
	}

	if len(invitedUsers) > 0 {
		go s.producer.PublishEventInvite(context.Background(), eventID, event.Title, routeData.Title, ownerID, inviterName, event.StartDate, invitedUsers)
	}
	return nil
}

func (s *EventService) GetParticipants(ctx context.Context, eventID uuid.UUID) ([]models.Participant, error) {
	return s.participantRepo.GetByEventID(ctx, eventID)
}

func (s *EventService) GetWaypoints(ctx context.Context, eventID uuid.UUID) ([]models.Waypoint, error) {
	return s.waypointRepo.GetByEventID(ctx, eventID)
}

func (s *EventService) GetInvitesByUserID(ctx context.Context, userID uuid.UUID) ([]models.Invite, error) {
	return s.participantRepo.GetInvitesByUserID(ctx, userID)
}

func (s *EventService) AddMedia(ctx context.Context, eventID, waypointID, userID uuid.UUID, req *models.AddMediaRequest) (*models.Media, error) {
	isParticipant, _ := s.participantRepo.IsParticipant(ctx, eventID, userID)
	if !isParticipant {
		return nil, ErrNotParticipant
	}
	media := &models.Media{
		ID: uuid.New(), EventID: eventID, WaypointID: waypointID, UserID: userID,
		MediaURL: req.MediaURL, ThumbnailURL: req.ThumbnailURL, MediumURL: req.MediumURL,
		MediaType: req.MediaType, Description: req.Description, CreatedAt: time.Now(),
	}
	return media, s.mediaRepo.Create(ctx, media)
}

func (s *EventService) GetMedia(ctx context.Context, waypointID uuid.UUID, page, size int) ([]models.Media, int64, error) {
	offset := (page - 1) * size
	return s.mediaRepo.GetByWaypointID(ctx, waypointID, size, offset)
}

func (s *EventService) DeleteMedia(ctx context.Context, mediaID, userID uuid.UUID) error {
	isOwner, _ := s.mediaRepo.IsOwner(ctx, mediaID, userID)
	if !isOwner {
		return errors.New("not authorized")
	}
	return s.mediaRepo.Delete(ctx, mediaID)
}

func (s *EventService) AddComment(ctx context.Context, eventID, waypointID, userID uuid.UUID, text string) (*models.Comment, error) {
	isParticipant, _ := s.participantRepo.IsParticipant(ctx, eventID, userID)
	if !isParticipant {
		return nil, ErrNotParticipant
	}
	comment := &models.Comment{ID: uuid.New(), EventID: eventID, WaypointID: waypointID, UserID: userID, Text: text, CreatedAt: time.Now()}
	return comment, s.commentRepo.Create(ctx, comment)
}

func (s *EventService) UpdateComment(ctx context.Context, commentID, userID uuid.UUID, text string) error {
	isAuthor, _ := s.commentRepo.IsAuthor(ctx, commentID, userID)
	if !isAuthor {
		return errors.New("not authorized")
	}
	return s.commentRepo.Update(ctx, &models.Comment{ID: commentID, Text: text})
}

func (s *EventService) DeleteComment(ctx context.Context, commentID, userID uuid.UUID) error {
	isAuthor, _ := s.commentRepo.IsAuthor(ctx, commentID, userID)
	if !isAuthor {
		return errors.New("not authorized")
	}
	return s.commentRepo.Delete(ctx, commentID)
}

func (s *EventService) IsEventOwner(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	return s.participantRepo.IsOwner(ctx, eventID, userID)
}

func (s *EventService) IsEventParticipant(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	return s.participantRepo.IsParticipant(ctx, eventID, userID)
}

func (s *EventService) GetUserEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.authClient.GetUserEmail(ctx, userID)
}

func (s *EventService) GetParticipantsStats(ctx context.Context, eventIDs []uuid.UUID) (map[uuid.UUID]eventdto.EventParticipantsStatsDTO, error) {
	stats, err := s.eventRepo.GetParticipantsStats(ctx, eventIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[uuid.UUID]eventdto.EventParticipantsStatsDTO)
	for id, stat := range stats {
		result[id] = eventdto.EventParticipantsStatsDTO{TotalInvites: stat.TotalInvites, TotalParticipants: stat.TotalParticipants}
	}
	return result, nil
}

func (s *EventService) CreateInviteLink(ctx context.Context, eventID, userID uuid.UUID, ttl time.Duration) (*eventdto.InviteLinkResponse, error) {
	isOwner, _ := s.participantRepo.IsOwner(ctx, eventID, userID)
	if !isOwner {
		return nil, ErrNotOwner
	}
	linkID := uuid.New().String()
	expiresAt := time.Now().Add(ttl)
	err := s.redisClient.Set(ctx, fmt.Sprintf("invite_link:%s", linkID), eventID.String(), ttl).Err()
	if err != nil {
		return nil, err
	}
	return &eventdto.InviteLinkResponse{LinkID: linkID, ExpiresAt: expiresAt}, nil
}

func (s *EventService) GetEventByInviteLink(ctx context.Context, linkID string) (*eventdto.EventPreviewResponse, error) {
	eventIDStr, err := s.redisClient.Get(ctx, fmt.Sprintf("invite_link:%s", linkID)).Result()
	if err != nil {
		return nil, ErrInviteLinkNotFound
	}
	eventID, _ := uuid.Parse(eventIDStr)
	event, _ := s.eventRepo.GetByID(ctx, eventID)
	if event == nil {
		return nil, ErrEventNotFound
	}
	participantsCount, _ := s.participantRepo.CountByEventID(ctx, eventID)
	waypoints, _ := s.waypointRepo.GetByEventID(ctx, eventID)
	waypointDTOs := make([]eventdto.WaypointPreviewResponse, len(waypoints))
	for i, wp := range waypoints {
		waypointDTOs[i] = eventdto.WaypointPreviewResponse{
			ID: wp.ID, City: wp.City, Description: wp.Description,
			Latitude: wp.Latitude, Longitude: wp.Longitude,
			PlannedStart: wp.PlannedStart, PlannedEnd: wp.PlannedEnd, OrderIndex: wp.OrderIndex,
		}
	}
	coverURL := ""
	if event.CoverURL.Valid {
		coverURL = event.CoverURL.String
	}
	return &eventdto.EventPreviewResponse{
		ID: event.ID, Title: event.Title, Description: event.Description,
		CoverURL: coverURL, StartDate: event.StartDate, EndDate: event.EndDate,
		IsGroup: event.IsGroup, OwnerID: event.OwnerID,
		ParticipantsCount: participantsCount, Waypoints: waypointDTOs,
	}, nil
}

func (s *EventService) UpdateEventCover(ctx context.Context, eventID, userID uuid.UUID, coverURL string) (*models.Event, error) {
	isOwner, _ := s.participantRepo.IsOwner(ctx, eventID, userID)
	if !isOwner {
		return nil, ErrNotOwner
	}
	event, _ := s.eventRepo.GetByID(ctx, eventID)
	if event == nil {
		return nil, ErrEventNotFound
	}
	event.CoverURL = sql.NullString{String: coverURL, Valid: coverURL != ""}
	event.UpdatedAt = timePtr(time.Now())
	return event, s.eventRepo.Update(ctx, event)
}

func (s *EventService) GetInviteLinkTTL() time.Duration {
	return s.cfg.InviteLinkTTL
}

// ========== СТАТУСЫ ПОЕЗДОК ==========

func (s *EventService) StartStatusUpdater(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.updateEventStatuses(ctx)
		}
	}
}

func (s *EventService) updateEventStatuses(ctx context.Context) {
	now := time.Now()
	s.activateEvents(ctx)
	activeEvents, _ := s.eventRepo.GetActiveEvents(ctx, now)
	for _, eventID := range activeEvents {
		s.completeEvent(ctx, eventID)
	}
}

func (s *EventService) activateEvents(ctx context.Context) error {
	plannedEvents, _ := s.eventRepo.GetPlannedEventsForActivation(ctx, time.Now())
	for _, eventID := range plannedEvents {
		s.eventRepo.UpdateStatus(ctx, eventID, "active")
	}
	activeEvents.Set(float64(len(plannedEvents)))
	return nil
}

func (s *EventService) completeEvent(ctx context.Context, eventID uuid.UUID) error {
	event, _ := s.eventRepo.GetByID(ctx, eventID)
	if event == nil {
		return nil
	}
	s.eventRepo.UpdateStatus(ctx, eventID, "completed")
	participants, _ := s.participantRepo.GetByEventID(ctx, eventID)
	participantIDs := make([]uuid.UUID, 0)
	for _, p := range participants {
		if p.IsActive() {
			participantIDs = append(participantIDs, p.UserID)
		}
	}
	routeData, _ := s.routeClient.GetRouteWithWaypoints(ctx, event.RouteID.String())
	countries := []string{}
	if routeData != nil && routeData.Country != "" {
		countries = []string{routeData.Country}
	}
	s.producer.PublishEventCompleted(ctx, eventID, event.RouteID, event.OwnerID, participantIDs, countries)
	eventsCompletedTotal.Inc()
	return nil
}
