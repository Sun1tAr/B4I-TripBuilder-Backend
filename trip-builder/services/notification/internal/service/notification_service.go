package service

import (
	"context"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/email"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/ratelimit"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/metrics"
	"github.com/google/uuid"
)

type NotificationService struct {
	notificationRepo repository.NotificationRepository
	deviceRepo       repository.DeviceRepository
	emailSender      *email.Sender
	templateEngine   *email.TemplateEngine
	emailRateLimiter *ratelimit.EmailRateLimiter
	log              *logger.Logger
	cfg              *config.Config
}

var notifMetrics = metrics.NewBusinessMetrics("notification")

var (
	notificationsSentTotal = notifMetrics.NewCounter("notifications_sent_total", "Total notifications sent")
	emailsSentTotal        = notifMetrics.NewCounter("emails_sent_total", "Total emails sent")
)

func NewNotificationService(
	notificationRepo repository.NotificationRepository,
	deviceRepo repository.DeviceRepository,
	emailSender *email.Sender,
	templateEngine *email.TemplateEngine,
	emailRateLimiter *ratelimit.EmailRateLimiter,
	log *logger.Logger,
	cfg *config.Config,
) *NotificationService {
	log.Debug("NotificationService: инициализация")
	return &NotificationService{
		notificationRepo: notificationRepo,
		deviceRepo:       deviceRepo,
		emailSender:      emailSender,
		templateEngine:   templateEngine,
		emailRateLimiter: emailRateLimiter,
		log:              log,
		cfg:              cfg,
	}
}

// CreateNotification создает in-app уведомление
func (s *NotificationService) CreateNotification(ctx context.Context, userID uuid.UUID, notificationType, title, body string, data models.JSONMap) (*models.Notification, error) {
	s.log.Debug("NotificationService.CreateNotification: начало",
		"user_id", userID,
		"type", notificationType,
	)

	notification := &models.Notification{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      notificationType,
		Title:     title,
		Body:      body,
		Data:      data,
		IsRead:    false,
		CreatedAt: time.Now(),
	}

	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		s.log.Error("NotificationService.CreateNotification: ошибка создания",
			"error", err,
			"user_id", userID,
		)
		return nil, err
	}

	s.log.Info("NotificationService.CreateNotification: уведомление создано",
		"notification_id", notification.ID,
		"user_id", userID,
		"type", notificationType,
	)

	notificationsSentTotal.Inc()
	return notification, nil
}

// CreateNotificationsBatch создает несколько in-app уведомлений
func (s *NotificationService) CreateNotificationsBatch(ctx context.Context, userIDs []uuid.UUID, notificationType, title, body string, data models.JSONMap) error {
	s.log.Debug("NotificationService.CreateNotificationsBatch: начало",
		"users_count", len(userIDs),
		"type", notificationType,
	)

	notifications := make([]*models.Notification, 0, len(userIDs))
	now := time.Now()

	for _, userID := range userIDs {
		// Копируем data для каждого уведомления
		notificationData := make(models.JSONMap)
		for k, v := range data {
			notificationData[k] = v
		}

		notification := &models.Notification{
			ID:        uuid.New(),
			UserID:    userID,
			Type:      notificationType,
			Title:     title,
			Body:      body,
			Data:      notificationData,
			IsRead:    false,
			CreatedAt: now,
		}
		notifications = append(notifications, notification)
	}

	if err := s.notificationRepo.CreateBatch(ctx, notifications); err != nil {
		s.log.Error("NotificationService.CreateNotificationsBatch: ошибка создания",
			"error", err,
		)
		return err
	}

	s.log.Info("NotificationService.CreateNotificationsBatch: уведомления созданы",
		"users_count", len(userIDs),
		"type", notificationType,
	)

	return nil
}

// GetNotifications возвращает уведомления пользователя с пагинацией
func (s *NotificationService) GetNotifications(ctx context.Context, userID uuid.UUID, page, size int) ([]*models.Notification, int64, error) {
	s.log.Debug("NotificationService.GetNotifications: начало",
		"user_id", userID,
		"page", page,
		"size", size,
	)

	offset := (page - 1) * size
	notifications, total, err := s.notificationRepo.GetByUserID(ctx, userID, size, offset)
	if err != nil {
		s.log.Error("NotificationService.GetNotifications: ошибка получения",
			"error", err,
			"user_id", userID,
		)
		return nil, 0, err
	}

	s.log.Debug("NotificationService.GetNotifications: уведомления получены",
		"user_id", userID,
		"count", len(notifications),
		"total", total,
	)

	return notifications, total, nil
}

// GetUnreadCount возвращает количество непрочитанных уведомлений
func (s *NotificationService) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	s.log.Debug("NotificationService.GetUnreadCount: начало",
		"user_id", userID,
	)

	count, err := s.notificationRepo.GetUnreadCount(ctx, userID)
	if err != nil {
		s.log.Error("NotificationService.GetUnreadCount: ошибка",
			"error", err,
			"user_id", userID,
		)
		return 0, err
	}

	s.log.Debug("NotificationService.GetUnreadCount: получено",
		"user_id", userID,
		"count", count,
	)

	return count, nil
}

// MarkAsRead отмечает уведомление как прочитанное
func (s *NotificationService) MarkAsRead(ctx context.Context, id, userID uuid.UUID) error {
	s.log.Debug("NotificationService.MarkAsRead: начало",
		"id", id,
		"user_id", userID,
	)

	if err := s.notificationRepo.MarkAsRead(ctx, id, userID); err != nil {
		s.log.Error("NotificationService.MarkAsRead: ошибка",
			"error", err,
			"id", id,
		)
		return err
	}

	s.log.Debug("NotificationService.MarkAsRead: отмечено как прочитанное",
		"id", id,
		"user_id", userID,
	)

	return nil
}

// MarkAllAsRead отмечает все уведомления пользователя как прочитанные
func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	s.log.Debug("NotificationService.MarkAllAsRead: начало",
		"user_id", userID,
	)

	count, err := s.notificationRepo.MarkAllAsRead(ctx, userID)
	if err != nil {
		s.log.Error("NotificationService.MarkAllAsRead: ошибка",
			"error", err,
			"user_id", userID,
		)
		return 0, err
	}

	s.log.Info("NotificationService.MarkAllAsRead: все отмечены как прочитанные",
		"user_id", userID,
		"count", count,
	)

	return count, nil
}

// RegisterDevice регистрирует устройство для push-уведомлений
func (s *NotificationService) RegisterDevice(ctx context.Context, userID uuid.UUID, token, platform string) error {
	s.log.Debug("NotificationService.RegisterDevice: начало",
		"user_id", userID,
		"platform", platform,
	)

	device := &models.UserDevice{
		ID:       uuid.New(),
		UserID:   userID,
		Token:    token,
		Platform: platform,
		LastUsed: time.Now(),
	}

	if err := s.deviceRepo.Register(ctx, device); err != nil {
		s.log.Error("NotificationService.RegisterDevice: ошибка",
			"error", err,
			"user_id", userID,
		)
		return err
	}

	s.log.Info("NotificationService.RegisterDevice: устройство зарегистрировано",
		"device_id", device.ID,
		"user_id", userID,
		"platform", platform,
	)

	return nil
}

// UnregisterDevice удаляет устройство
func (s *NotificationService) UnregisterDevice(ctx context.Context, userID uuid.UUID, token string) error {
	s.log.Debug("NotificationService.UnregisterDevice: начало",
		"user_id", userID,
	)

	if err := s.deviceRepo.Unregister(ctx, userID, token); err != nil {
		s.log.Error("NotificationService.UnregisterDevice: ошибка",
			"error", err,
			"user_id", userID,
		)
		return err
	}

	s.log.Info("NotificationService.UnregisterDevice: устройство удалено",
		"user_id", userID,
	)

	return nil
}

// SendWelcomeEmail отправляет приветственное письмо
func (s *NotificationService) SendWelcomeEmail(ctx context.Context, userID uuid.UUID, emailAddress, firstName, lastName string) {
	s.log.Debug("NotificationService.SendWelcomeEmail: начало",
		"user_id", userID,
		"email", emailAddress,
	)

	// Проверяем rate limit
	allowed, err := s.emailRateLimiter.Allow(ctx, userID.String())
	if err != nil {
		s.log.Error("NotificationService.SendWelcomeEmail: ошибка проверки rate limit",
			"error", err,
			"user_id", userID,
		)
		return
	}
	if !allowed {
		s.log.Info("NotificationService.SendWelcomeEmail: превышен лимит",
			"user_id", userID,
		)
		return
	}

	// Рендерим шаблон
	data := &email.RenderData{
		FirstName: firstName,
		LastName:  lastName,
		Email:     emailAddress,
	}
	subject, body, err := s.templateEngine.Render(ctx, models.TemplateTypeWelcome, "ru", data)
	if err != nil {
		s.log.Error("NotificationService.SendWelcomeEmail: ошибка рендеринга",
			"error", err,
			"user_id", userID,
		)
		return
	}

	// Отправляем письмо асинхронно
	s.emailSender.SendAsync(ctx, emailAddress, subject, body)

	s.log.Info("NotificationService.SendWelcomeEmail: письмо отправлено в очередь",
		"user_id", userID,
		"email", emailAddress,
	)
	emailsSentTotal.Inc()
}

// truncateString обрезает строку до указанной длины
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// SendVerificationEmail отправляет письмо с подтверждением email
func (s *NotificationService) SendVerificationEmail(ctx context.Context, userID uuid.UUID, emailAddress, firstName, lastName, verificationToken string) {
	s.log.Debug("NotificationService.SendVerificationEmail: начало",
		"user_id", userID,
		"email", emailAddress,
		"verification_token", verificationToken,
		"token_length", len(verificationToken),
	)

	// Проверяем, что токен не пустой
	if verificationToken == "" {
		s.log.Error("NotificationService.SendVerificationEmail: токен пустой!", "user_id", userID)
		return
	}

	// Проверяем rate limit
	allowed, err := s.emailRateLimiter.Allow(ctx, userID.String())
	if err != nil {
		s.log.Error("NotificationService.SendVerificationEmail: ошибка проверки rate limit",
			"error", err, "user_id", userID)
		return
	}
	if !allowed {
		s.log.Info("NotificationService.SendVerificationEmail: превышен лимит", "user_id", userID)
		return
	}

	// Формируем ссылку подтверждения через конфиг
	verificationLink := s.cfg.GetVerifyEmailURL(verificationToken)

	s.log.Debug("NotificationService.SendVerificationEmail: ссылка сформирована",
		"verification_link", verificationLink,
	)

	// Рендерим шаблон
	data := &email.RenderData{
		FirstName:        firstName,
		LastName:         lastName,
		Email:            emailAddress,
		VerificationLink: verificationLink,
	}

	subject, body, err := s.templateEngine.Render(ctx, "user.verification", "ru", data)
	if err != nil {
		s.log.Error("NotificationService.SendVerificationEmail: ошибка рендеринга",
			"error", err, "user_id", userID)
		return
	}

	// Отправляем письмо асинхронно
	s.emailSender.SendAsync(ctx, emailAddress, subject, body)

	s.log.Info("NotificationService.SendVerificationEmail: письмо отправлено в очередь",
		"user_id", userID,
		"email", emailAddress,
	)
}

// SendEventInviteEmail отправляет письмо с приглашением в поездку
func (s *NotificationService) SendEventInviteEmail(ctx context.Context, userID uuid.UUID, emailAddress, firstName string, eventTitle, routeTitle, inviterName, startDate string, eventID uuid.UUID) {
	s.log.Debug("NotificationService.SendEventInviteEmail: начало",
		"user_id", userID,
		"event_id", eventID,
	)

	allowed, err := s.emailRateLimiter.Allow(ctx, userID.String())
	if err != nil || !allowed {
		if err != nil {
			s.log.Error("NotificationService.SendEventInviteEmail: ошибка rate limit", "error", err)
		}
		return
	}

	// Формируем ссылку приглашения через конфиг
	inviteLink := s.cfg.GetEventInviteURL(eventID.String())

	data := &email.RenderData{
		FirstName:      firstName,
		EventTitle:     eventTitle,
		RouteTitle:     routeTitle,
		InviterName:    inviterName,
		EventStartDate: startDate,
		InviteLink:     inviteLink,
	}

	subject, body, err := s.templateEngine.Render(ctx, models.TemplateTypeEventInvite, "ru", data)
	if err != nil {
		s.log.Error("NotificationService.SendEventInviteEmail: ошибка рендеринга", "error", err)
		return
	}

	s.emailSender.SendAsync(ctx, emailAddress, subject, body)

	s.log.Info("NotificationService.SendEventInviteEmail: письмо отправлено в очередь",
		"user_id", userID,
		"event_id", eventID,
	)
}

// SendTaskAssignedEmail отправляет письмо о назначении задачи
func (s *NotificationService) SendTaskAssignedEmail(ctx context.Context, userID uuid.UUID, emailAddress, firstName string, taskTitle, taskDescription, eventTitle, dueDate, priority string, taskID, eventID uuid.UUID) {
	s.log.Debug("NotificationService.SendTaskAssignedEmail: начало",
		"user_id", userID,
		"task_id", taskID,
	)

	allowed, err := s.emailRateLimiter.Allow(ctx, userID.String())
	if err != nil || !allowed {
		return
	}

	// Формируем ссылку на задачу через конфиг
	taskLink := s.cfg.GetTaskURL(eventID.String(), taskID.String())

	data := &email.RenderData{
		FirstName:       firstName,
		TaskTitle:       taskTitle,
		TaskDescription: taskDescription,
		EventTitle:      eventTitle,
		TaskDueDate:     dueDate,
		TaskPriority:    priority,
		TaskLink:        taskLink,
	}

	subject, body, err := s.templateEngine.Render(ctx, models.TemplateTypeTaskAssigned, "ru", data)
	if err != nil {
		s.log.Error("NotificationService.SendTaskAssignedEmail: ошибка рендеринга", "error", err)
		return
	}

	s.emailSender.SendAsync(ctx, emailAddress, subject, body)

	s.log.Info("NotificationService.SendTaskAssignedEmail: письмо отправлено в очередь",
		"user_id", userID,
		"task_id", taskID,
	)
}

// SendEventReminderEmail отправляет напоминание о поездке
func (s *NotificationService) SendEventReminderEmail(ctx context.Context, userID uuid.UUID, emailAddress, firstName string, eventTitle, startDate string, daysLeft int, eventID uuid.UUID) {
	s.log.Debug("NotificationService.SendEventReminderEmail: начало",
		"user_id", userID,
		"event_id", eventID,
		"days_left", daysLeft,
	)

	allowed, err := s.emailRateLimiter.Allow(ctx, userID.String())
	if err != nil || !allowed {
		return
	}

	// Формируем ссылку на поездку через конфиг
	eventLink := s.cfg.GetEventURL(eventID.String())

	data := &email.RenderData{
		FirstName:      firstName,
		EventTitle:     eventTitle,
		EventStartDate: startDate,
		DaysLeft:       daysLeft,
		EventLink:      eventLink,
	}

	subject, body, err := s.templateEngine.Render(ctx, models.TemplateTypeEventReminder, "ru", data)
	if err != nil {
		s.log.Error("NotificationService.SendEventReminderEmail: ошибка рендеринга", "error", err)
		return
	}

	s.emailSender.SendAsync(ctx, emailAddress, subject, body)

	s.log.Info("NotificationService.SendEventReminderEmail: письмо отправлено в очередь",
		"user_id", userID,
		"event_id", eventID,
	)
}

// SendChatMessageEmail отправляет уведомление о новом сообщении в чате
func (s *NotificationService) SendChatMessageEmail(ctx context.Context, userID uuid.UUID, emailAddress, firstName string, eventTitle, senderName, messageText string, eventID uuid.UUID) {
	s.log.Debug("NotificationService.SendChatMessageEmail: начало",
		"user_id", userID,
		"event_id", eventID,
	)

	allowed, err := s.emailRateLimiter.Allow(ctx, userID.String())
	if err != nil || !allowed {
		return
	}

	// Формируем ссылку на чат через конфиг
	chatLink := s.cfg.GetChatURL(eventID.String())

	data := &email.RenderData{
		FirstName:   firstName,
		EventTitle:  eventTitle,
		SenderName:  senderName,
		MessageText: truncateString(messageText, 200),
		ChatLink:    chatLink,
	}

	subject, body, err := s.templateEngine.Render(ctx, models.TemplateTypeChatMessage, "ru", data)
	if err != nil {
		s.log.Error("NotificationService.SendChatMessageEmail: ошибка рендеринга", "error", err)
		return
	}

	s.emailSender.SendAsync(ctx, emailAddress, subject, body)

	s.log.Info("NotificationService.SendChatMessageEmail: письмо отправлено в очередь",
		"user_id", userID,
		"event_id", eventID,
	)
}

func (s *NotificationService) SendPasswordResetEmail(ctx context.Context, userID uuid.UUID, emailAddress, firstName, resetToken string) {
	s.log.Debug("NotificationService.SendPasswordResetEmail: начало",
		"user_id", userID, "email", emailAddress,
	)

	allowed, err := s.emailRateLimiter.Allow(ctx, userID.String())
	if err != nil || !allowed {
		return
	}

	resetLink := s.cfg.GetForgotPasswordURL(resetToken)

	data := &email.RenderData{
		FirstName: firstName,
		Email:     emailAddress,
		ResetLink: resetLink,
	}

	subject, body, err := s.templateEngine.Render(ctx, "password.reset", "ru", data)
	if err != nil {
		return
	}

	s.emailSender.SendAsync(ctx, emailAddress, subject, body)
}
