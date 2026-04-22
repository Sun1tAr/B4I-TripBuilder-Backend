package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/client/event"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/client/user"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

var (
	ErrTaskNotFound        = errors.New("task not found")
	ErrNotAuthorized       = errors.New("not authorized to modify this task")
	ErrInvalidStatus       = errors.New("invalid task status")
	ErrInvalidPriority     = errors.New("invalid priority value")
	ErrInvalidDueDate      = errors.New("due date cannot be in the past")
	ErrEventNotFound       = errors.New("event not found")
	ErrTemplateNotFound    = errors.New("template not found")
	ErrNotApproved         = errors.New("task not approved by admin")
	ErrAlreadyApproved     = errors.New("task already approved")
	ErrNotEventOwner       = errors.New("user is not the owner of this event")
	ErrNotEventParticipant = errors.New("user is not a participant of this event")
)

type TaskService struct {
	taskRepo     repository.TaskRepository
	commentRepo  repository.TaskCommentRepository
	templateRepo repository.TaskTemplateRepository
	eventClient  *event.Client
	userClient   *user.Client
	producer     *kafka.TaskProducer
	log          *logger.Logger
}

func NewTaskService(
	taskRepo repository.TaskRepository,
	commentRepo repository.TaskCommentRepository,
	templateRepo repository.TaskTemplateRepository,
	eventClient *event.Client,
	userClient *user.Client,
	producer *kafka.TaskProducer,
	log *logger.Logger,
) *TaskService {
	return &TaskService{
		taskRepo:     taskRepo,
		commentRepo:  commentRepo,
		templateRepo: templateRepo,
		eventClient:  eventClient,
		userClient:   userClient,
		producer:     producer,
		log:          log,
	}
}

// CreateTask создаёт новую задачу
func (s *TaskService) CreateTask(ctx context.Context, req *models.CreateTaskRequest, userID uuid.UUID) (*models.Task, error) {
	s.log.Debug("CreateTask: начало", "user_id", userID, "event_id", req.EventID, "title", req.Title)

	// Валидация приоритета
	if req.Priority != "" && req.Priority != "high" && req.Priority != "medium" && req.Priority != "low" {
		return nil, ErrInvalidPriority
	}

	// Валидация даты
	if req.DueDate != nil && req.DueDate.Before(time.Now()) {
		return nil, ErrInvalidDueDate
	}

	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}

	// Проверяем права: владелец или участник
	isOwner, err := s.eventClient.IsEventOwner(ctx, req.EventID, userID)
	if err != nil {
		return nil, err
	}

	isParticipant, err := s.eventClient.IsEventParticipant(ctx, req.EventID, userID)
	if err != nil {
		return nil, err
	}

	if !isOwner && !isParticipant {
		s.log.Warn("CreateTask: пользователь не является участником события", "user_id", userID, "event_id", req.EventID)
		return nil, ErrNotAuthorized
	}

	// Получаем информацию о событии (нужен ownerID)
	event, err := s.eventClient.GetEvent(ctx, req.EventID)
	if err != nil {
		s.log.Error("CreateTask: ошибка получения события", "error", err, "event_id", req.EventID)
		return nil, err
	}
	ownerID := event.OwnerID

	// Определяем, требует ли задача подтверждения администратора
	approvedByAdmin := false

	if isOwner {
		// Владелец поездки: любые задачи сразу подтверждены
		approvedByAdmin = true
		s.log.Debug("CreateTask: владелец создаёт задачу, сразу подтверждена")
	} else {
		// Обычный участник
		if req.AssignedTo == nil {
			// Общая задача (назначена на всех) – требует подтверждения владельца
			approvedByAdmin = false
			s.log.Debug("CreateTask: общая задача, требует подтверждения владельца")
		} else if *req.AssignedTo == userID {
			// Задача на себя – сразу подтверждена
			approvedByAdmin = true
			s.log.Debug("CreateTask: задача на себя, сразу подтверждена")
		} else if *req.AssignedTo == ownerID {
			// Задача на владельца – требует подтверждения
			approvedByAdmin = false
			s.log.Debug("CreateTask: задача на владельца, требует подтверждения")
		} else {
			// Задача на другого участника – требует подтверждения владельца
			approvedByAdmin = false
			s.log.Debug("CreateTask: задача на другого участника, требует подтверждения")
		}
	}

	now := time.Now()
	task := &models.Task{
		ID:              uuid.New(),
		EventID:         req.EventID,
		Title:           req.Title,
		Description:     req.Description,
		AssignedTo:      req.AssignedTo,
		CreatedBy:       userID,
		Priority:        priority,
		Status:          "pending",
		DueDate:         req.DueDate,
		ApprovedByAdmin: approvedByAdmin,
		CreatedAt:       now,
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		s.log.Error("CreateTask: ошибка сохранения", "error", err)
		return nil, err
	}

	s.log.Info("CreateTask: задача создана", "task_id", task.ID, "approved_by_admin", approvedByAdmin)

	// Если задача назначена на конкретного пользователя — публикуем событие
	// ВАЖНО: используем context.Background() чтобы не зависеть от HTTP-контекста
	if task.AssignedTo != nil && *task.AssignedTo != uuid.Nil && *task.AssignedTo != userID {
		go func() {
			s.publishTaskAssigned(context.Background(), task, userID, event)
		}()
	}

	return task, nil
}

// GetTaskByID возвращает задачу по ID
func (s *TaskService) GetTaskByID(ctx context.Context, taskID uuid.UUID, userID *uuid.UUID) (*models.Task, error) {
	s.log.Debug("GetTaskByID: начало", "task_id", taskID, "user_id", userID)

	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		s.log.Error("GetTaskByID: ошибка БД", "error", err, "task_id", taskID)
		return nil, err
	}
	if task == nil {
		s.log.Debug("GetTaskByID: задача не найдена", "task_id", taskID)
		return nil, ErrTaskNotFound
	}

	s.log.Trace("GetTaskByID: задача найдена", "task_id", taskID, "status", task.Status)
	return task, nil
}

// GetTasks возвращает список задач с учётом прав пользователя
func (s *TaskService) GetTasks(ctx context.Context, eventID *uuid.UUID, userID uuid.UUID, page, size int) ([]models.Task, int64, error) {
	s.log.Debug("GetTasks: начало", "user_id", userID, "event_id", eventID, "page", page, "size", size)

	// Проверяем, является ли пользователь владельцем события
	isOwner := false
	if eventID != nil {
		owner, err := s.eventClient.IsEventOwner(ctx, *eventID, userID)
		if err != nil {
			s.log.Error("GetTasks: ошибка проверки владельца", "error", err)
			// не фатально, считаем что не владелец
		} else {
			isOwner = owner
		}
	}

	offset := (page - 1) * size
	tasks, total, err := s.taskRepo.List(ctx, eventID, userID, isOwner, size, offset)
	if err != nil {
		s.log.Error("GetTasks: ошибка получения списка", "error", err)
		return nil, 0, err
	}

	s.log.Info("GetTasks: список получен", "count", len(tasks), "total", total, "is_owner", isOwner)
	return tasks, total, nil
}

// ApproveTask подтверждает задачу (только для админа)
func (s *TaskService) ApproveTask(ctx context.Context, taskID, userID uuid.UUID) (*models.Task, error) {
	s.log.Debug("ApproveTask: начало", "task_id", taskID, "user_id", userID)

	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrTaskNotFound
	}

	// Проверяем, что пользователь — владелец события
	isOwner, err := s.eventClient.IsEventOwner(ctx, task.EventID, userID)
	if err != nil {
		s.log.Error("ApproveTask: ошибка проверки владельца", "error", err)
		return nil, err
	}
	if !isOwner {
		s.log.Warn("ApproveTask: пользователь не является владельцем", "user_id", userID, "event_id", task.EventID)
		return nil, ErrNotAuthorized
	}

	// Проверяем, что задача еще не подтверждена
	if task.ApprovedByAdmin {
		s.log.Debug("ApproveTask: задача уже подтверждена", "task_id", taskID)
		return nil, ErrAlreadyApproved
	}

	task.ApprovedByAdmin = true
	now := time.Now()
	task.UpdatedAt = &now

	if err := s.taskRepo.Update(ctx, task); err != nil {
		s.log.Error("ApproveTask: ошибка сохранения", "error", err, "task_id", taskID)
		return nil, err
	}

	s.log.Info("ApproveTask: задача подтверждена", "task_id", taskID, "user_id", userID)
	return task, nil
}

// UpdateTask обновляет задачу
func (s *TaskService) UpdateTask(ctx context.Context, taskID uuid.UUID, req *models.UpdateTaskRequest, userID uuid.UUID) (*models.Task, error) {
	s.log.Debug("UpdateTask: начало",
		"task_id", taskID,
		"user_id", userID,
	)

	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		s.log.Error("UpdateTask: ошибка получения задачи", "error", err, "task_id", taskID)
		return nil, err
	}
	if task == nil {
		s.log.Debug("UpdateTask: задача не найдена", "task_id", taskID)
		return nil, ErrTaskNotFound
	}

	// Проверяем права: владелец события, создатель задачи или исполнитель
	isOwner, err := s.eventClient.IsEventOwner(ctx, task.EventID, userID)
	if err != nil {
		s.log.Error("UpdateTask: ошибка проверки владельца", "error", err, "task_id", taskID)
		return nil, err
	}

	isCreator := task.CreatedBy == userID
	isAssigned := task.AssignedTo != nil && *task.AssignedTo == userID

	s.log.Debug("UpdateTask: проверка прав",
		"task_id", taskID,
		"is_owner", isOwner,
		"is_creator", isCreator,
		"is_assigned", isAssigned,
		"approved_by_admin", task.ApprovedByAdmin,
	)

	// Если задача не подтверждена администратором, менять может только создатель или владелец
	if !task.ApprovedByAdmin && !isCreator && !isOwner {
		s.log.Warn("UpdateTask: попытка редактирования неподтвержденной задачи",
			"task_id", taskID,
			"user_id", userID,
		)
		return nil, ErrNotAuthorized
	}

	// Запоминаем старого исполнителя для проверки изменений
	oldAssignedTo := task.AssignedTo

	// Флаг для отслеживания изменений статуса
	oldStatus := task.Status
	statusChangedToDone := false

	// Обновляем Title (могут владелец или создатель)
	if req.Title != nil {
		if !isOwner && !isCreator {
			s.log.Warn("UpdateTask: попытка изменения title без прав", "task_id", taskID, "user_id", userID)
			return nil, ErrNotAuthorized
		}
		task.Title = *req.Title
		s.log.Debug("UpdateTask: title обновлен", "task_id", taskID, "new_title", task.Title)
	}

	// Обновляем Description (могут владелец или создатель)
	if req.Description != nil {
		if !isOwner && !isCreator {
			s.log.Warn("UpdateTask: попытка изменения description без прав", "task_id", taskID, "user_id", userID)
			return nil, ErrNotAuthorized
		}
		task.Description = *req.Description
		s.log.Debug("UpdateTask: description обновлен", "task_id", taskID)
	}

	// Обновляем Priority (могут владелец или создатель)
	if req.Priority != nil {
		if !isOwner && !isCreator {
			s.log.Warn("UpdateTask: попытка изменения priority без прав", "task_id", taskID, "user_id", userID)
			return nil, ErrNotAuthorized
		}
		if *req.Priority != "high" && *req.Priority != "medium" && *req.Priority != "low" {
			return nil, ErrInvalidPriority
		}
		task.Priority = *req.Priority
		s.log.Debug("UpdateTask: priority обновлен", "task_id", taskID, "new_priority", task.Priority)
	}

	// Обновляем DueDate (могут владелец или создатель)
	if req.DueDate != nil {
		if !isOwner && !isCreator {
			s.log.Warn("UpdateTask: попытка изменения due_date без прав", "task_id", taskID, "user_id", userID)
			return nil, ErrNotAuthorized
		}
		if req.DueDate.Before(time.Now()) {
			return nil, ErrInvalidDueDate
		}
		task.DueDate = req.DueDate
		s.log.Debug("UpdateTask: due_date обновлен", "task_id", taskID, "new_due_date", task.DueDate)
	}

	// Обновляем AssignedTo (только владелец или создатель)
	if req.AssignedTo != nil {
		if !isOwner && !isCreator {
			s.log.Warn("UpdateTask: попытка переназначения без прав", "task_id", taskID, "user_id", userID)
			return nil, ErrNotAuthorized
		}
		task.AssignedTo = req.AssignedTo
		s.log.Debug("UpdateTask: assigned_to обновлен",
			"task_id", taskID,
			"old_assigned_to", oldAssignedTo,
			"new_assigned_to", task.AssignedTo,
		)
	}

	// Обновляем Status (могут владелец, создатель или исполнитель)
	if req.Status != nil {
		if !isOwner && !isCreator && !isAssigned {
			s.log.Warn("UpdateTask: попытка изменения статуса без прав", "task_id", taskID, "user_id", userID)
			return nil, ErrNotAuthorized
		}

		if *req.Status != "pending" && *req.Status != "done" && *req.Status != "cancelled" {
			return nil, ErrInvalidStatus
		}

		task.Status = *req.Status
		s.log.Debug("UpdateTask: status обновлен", "task_id", taskID, "old_status", oldStatus, "new_status", task.Status)

		if task.Status == "done" && oldStatus != "done" {
			now := time.Now()
			task.CompletedAt = &now
			statusChangedToDone = true
			s.log.Info("UpdateTask: задача выполнена", "task_id", taskID, "user_id", userID)
		}

		if task.Status == "cancelled" {
			s.log.Info("UpdateTask: задача отменена", "task_id", taskID, "user_id", userID)
		}
	}

	// Обновляем updated_at
	now := time.Now()
	task.UpdatedAt = &now

	// Сохраняем изменения в БД
	if err := s.taskRepo.Update(ctx, task); err != nil {
		s.log.Error("UpdateTask: ошибка сохранения", "error", err, "task_id", taskID)
		return nil, err
	}

	s.log.Debug("UpdateTask: задача сохранена в БД", "task_id", taskID)

	// Получаем информацию о событии для Kafka
	event, err := s.eventClient.GetEvent(ctx, task.EventID)
	if err != nil {
		s.log.Error("UpdateTask: ошибка получения события для Kafka", "error", err, "event_id", task.EventID)
	}

	// Если изменился исполнитель — публикуем событие task.assigned
	if req.AssignedTo != nil && *req.AssignedTo != uuid.Nil {
		shouldPublish := false
		if oldAssignedTo == nil {
			shouldPublish = true
		} else if *oldAssignedTo != *req.AssignedTo {
			shouldPublish = true
		}

		if shouldPublish {
			s.log.Info("UpdateTask: исполнитель изменен, публикуем task.assigned",
				"task_id", taskID,
				"old_assigned_to", oldAssignedTo,
				"new_assigned_to", *req.AssignedTo,
			)
			// Используем горутину с context.Background() чтобы не зависеть от HTTP-контекста
			go func() {
				s.publishTaskAssigned(context.Background(), task, userID, event)
			}()
		}
	}

	// Если задача выполнена — публикуем task.completed
	if statusChangedToDone {
		s.log.Info("UpdateTask: задача выполнена, публикуем task.completed",
			"task_id", taskID,
			"event_id", task.EventID,
		)
		go func() {
			if err := s.producer.PublishTaskCompleted(
				context.Background(),
				taskID,
				task.EventID,
				userID,
				task.Title,
			); err != nil {
				s.log.Error("UpdateTask: ошибка публикации task.completed",
					"error", err,
					"task_id", taskID,
				)
			} else {
				s.log.Info("UpdateTask: событие task.completed опубликовано", "task_id", taskID)
			}
		}()
	}

	s.log.Info("UpdateTask: задача успешно обновлена",
		"task_id", taskID,
		"user_id", userID,
		"new_status", task.Status,
	)

	return task, nil
}

// DeleteTask удаляет задачу
func (s *TaskService) DeleteTask(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) error {
	s.log.Debug("DeleteTask: начало", "task_id", taskID, "user_id", userID)

	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return ErrTaskNotFound
	}

	// Проверяем права: админ или создатель
	isOwner, err := s.eventClient.IsEventOwner(ctx, task.EventID, userID)
	if err != nil {
		s.log.Error("DeleteTask: ошибка проверки владельца", "error", err)
		return err
	}

	isCreator := task.CreatedBy == userID

	if !isOwner && !isCreator {
		s.log.Warn("DeleteTask: попытка удаления без прав", "task_id", taskID, "user_id", userID)
		return ErrNotAuthorized
	}

	if err := s.taskRepo.Delete(ctx, taskID); err != nil {
		s.log.Error("DeleteTask: ошибка удаления", "error", err, "task_id", taskID)
		return err
	}

	s.log.Info("DeleteTask: задача удалена", "task_id", taskID, "user_id", userID)
	return nil
}

// AddComment добавляет комментарий к задаче
func (s *TaskService) AddComment(ctx context.Context, taskID uuid.UUID, userID uuid.UUID, text string) (*models.TaskComment, error) {
	s.log.Debug("AddComment: начало", "task_id", taskID, "user_id", userID)

	// Проверяем, существует ли задача
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrTaskNotFound
	}

	comment := &models.TaskComment{
		ID:        uuid.New(),
		TaskID:    taskID,
		UserID:    userID,
		Text:      text,
		CreatedAt: time.Now(),
	}

	s.log.Trace("AddComment: сохранение комментария", "comment_id", comment.ID, "task_id", taskID)

	if err := s.commentRepo.Create(ctx, comment); err != nil {
		s.log.Error("AddComment: ошибка сохранения", "error", err, "comment_id", comment.ID)
		return nil, err
	}

	s.log.Info("AddComment: комментарий добавлен", "comment_id", comment.ID, "task_id", taskID, "user_id", userID)
	return comment, nil
}

// GetComments возвращает комментарии к задаче
func (s *TaskService) GetComments(ctx context.Context, taskID uuid.UUID, page, size int) ([]models.TaskComment, int64, error) {
	s.log.Debug("GetComments: начало", "task_id", taskID, "page", page, "size", size)

	offset := (page - 1) * size
	comments, total, err := s.commentRepo.GetByTaskID(ctx, taskID, size, offset)
	if err != nil {
		s.log.Error("GetComments: ошибка получения", "error", err, "task_id", taskID)
		return nil, 0, err
	}

	s.log.Trace("GetComments: комментарии получены", "task_id", taskID, "count", len(comments), "total", total)
	return comments, total, nil
}

// DeleteComment удаляет комментарий
func (s *TaskService) DeleteComment(ctx context.Context, commentID uuid.UUID, userID uuid.UUID) error {
	s.log.Debug("DeleteComment: начало", "comment_id", commentID, "user_id", userID)

	isAuthor, err := s.commentRepo.IsAuthor(ctx, commentID, userID)
	if err != nil {
		return err
	}
	if !isAuthor {
		s.log.Warn("DeleteComment: попытка удаления чужого комментария", "comment_id", commentID, "user_id", userID)
		return ErrNotAuthorized
	}

	if err := s.commentRepo.Delete(ctx, commentID); err != nil {
		s.log.Error("DeleteComment: ошибка удаления", "error", err, "comment_id", commentID)
		return err
	}

	s.log.Info("DeleteComment: комментарий удалён", "comment_id", commentID, "user_id", userID)
	return nil
}

// GetTemplates возвращает список шаблонов задач
func (s *TaskService) GetTemplates(ctx context.Context) ([]models.TaskTemplate, error) {
	s.log.Debug("GetTemplates: начало")

	templates, err := s.templateRepo.List(ctx)
	if err != nil {
		s.log.Error("GetTemplates: ошибка получения", "error", err)
		return nil, err
	}

	s.log.Info("GetTemplates: шаблоны получены", "count", len(templates))
	return templates, nil
}

// GenerateTasksFromTemplates генерирует задачи для события по шаблонам
func (s *TaskService) GenerateTasksFromTemplates(ctx context.Context, eventID uuid.UUID, startDate time.Time, createdBy uuid.UUID) ([]interface{}, error) {
	s.log.Debug("GenerateTasksFromTemplates: начало", "event_id", eventID, "start_date", startDate, "created_by", createdBy)

	templates, err := s.templateRepo.List(ctx)
	if err != nil {
		s.log.Error("GenerateTasksFromTemplates: ошибка получения шаблонов", "error", err)
		return nil, err
	}

	s.log.Trace("GenerateTasksFromTemplates: получено шаблонов", "count", len(templates))

	var tasks []interface{}
	now := time.Now()

	for _, template := range templates {
		// Рассчитываем due_date на основе trigger_days
		dueDate := startDate.AddDate(0, 0, template.TriggerDays)

		task := &models.Task{
			ID:              uuid.New(),
			EventID:         eventID,
			Title:           template.Title,
			Description:     template.Description,
			AssignedTo:      nil,
			CreatedBy:       createdBy,
			Priority:        template.Priority,
			Status:          "pending",
			DueDate:         &dueDate,
			ApprovedByAdmin: true, // задачи от админа сразу подтверждены
			CreatedAt:       now,
		}

		s.log.Trace("GenerateTasksFromTemplates: создание задачи по шаблону", "template_id", template.ID, "task_id", task.ID, "due_date", dueDate)

		if err := s.taskRepo.Create(ctx, task); err != nil {
			s.log.Error("GenerateTasksFromTemplates: ошибка создания задачи", "error", err, "template_id", template.ID)
			continue
		}
		tasks = append(tasks, *task)
	}

	s.log.Info("GenerateTasksFromTemplates: задачи сгенерированы", "event_id", eventID, "created_count", len(tasks), "total_templates", len(templates))
	return tasks, nil
}

// RecalculateDueDates пересчитывает дедлайны задач при изменении даты события
func (s *TaskService) RecalculateDueDates(ctx context.Context, eventID uuid.UUID, newStartDate time.Time) error {
	s.log.Debug("RecalculateDueDates: начало", "event_id", eventID, "new_start_date", newStartDate)

	// Получаем все незавершённые задачи события
	tasks, err := s.taskRepo.GetByEventID(ctx, eventID)
	if err != nil {
		s.log.Error("RecalculateDueDates: ошибка получения задач", "error", err, "event_id", eventID)
		return err
	}

	updatedCount := 0
	for _, task := range tasks {
		// Пропускаем выполненные и отменённые задачи
		if task.Status == "done" || task.Status == "cancelled" {
			continue
		}

		if task.DueDate == nil {
			continue
		}

		// Для простоты: пересчитываем дедлайн как новый старт + смещение от старого старта
		// В реальном сценарии нужно знать исходный start_date события
		// TODO: при получении события event.updated будет старый и новый start_date
		s.log.Trace("RecalculateDueDates: задача требует пересчёта", "task_id", task.ID, "old_due_date", task.DueDate)
		updatedCount++
	}

	s.log.Info("RecalculateDueDates: пересчёт завершён", "event_id", eventID, "tasks_checked", len(tasks), "updated", updatedCount)
	return nil
}

// CancelEventTasks отменяет все задачи события
func (s *TaskService) CancelEventTasks(ctx context.Context, eventID uuid.UUID) error {
	s.log.Debug("CancelEventTasks: начало", "event_id", eventID)

	if err := s.taskRepo.CancelByEventID(ctx, eventID); err != nil {
		s.log.Error("CancelEventTasks: ошибка отмены задач", "error", err, "event_id", eventID)
		return err
	}

	s.log.Info("CancelEventTasks: задачи отменены", "event_id", eventID)
	return nil
}

// GetCommentsCount возвращает количество комментариев у задачи
func (s *TaskService) GetCommentsCount(ctx context.Context, taskID uuid.UUID) (int64, error) {
	s.log.Trace("GetCommentsCount: начало", "task_id", taskID)
	return s.commentRepo.CountByTaskID(ctx, taskID)
}

// publishTaskAssigned публикует событие о назначении задачи
// Принимает уже полученный event, чтобы не делать лишний запрос
func (s *TaskService) publishTaskAssigned(ctx context.Context, task *models.Task, assignedBy uuid.UUID, event *event.Event) {
	s.log.Debug("publishTaskAssigned: начало",
		"task_id", task.ID,
		"assigned_to", task.AssignedTo,
		"assigned_by", assignedBy,
	)

	if task.AssignedTo == nil || *task.AssignedTo == uuid.Nil {
		s.log.Debug("publishTaskAssigned: задача не назначена, пропускаем")
		return
	}

	// Получаем профиль исполнителя
	var assignedEmail, assignedName string
	assignedProfile, err := s.userClient.GetUserByID(ctx, *task.AssignedTo)
	if err != nil {
		s.log.Error("publishTaskAssigned: ошибка получения профиля исполнителя",
			"error", err,
			"user_id", *task.AssignedTo,
		)
		// Fallback
		assignedEmail = fmt.Sprintf("user-%s@tripbuilder.local", task.AssignedTo.String()[:8])
		assignedName = "Участник"
	} else if assignedProfile != nil {
		assignedName = fmt.Sprintf("%s %s", assignedProfile.FirstName, assignedProfile.LastName)
		if assignedName == " " {
			assignedName = "Участник"
		}
		assignedEmail = assignedProfile.Email
		if assignedEmail == "" {
			assignedEmail = fmt.Sprintf("user-%s@tripbuilder.local", task.AssignedTo.String()[:8])
		}
	} else {
		assignedEmail = fmt.Sprintf("user-%s@tripbuilder.local", task.AssignedTo.String()[:8])
		assignedName = "Участник"
	}

	// Получаем профиль назначившего
	var assignedByName string
	assignedByProfile, err := s.userClient.GetUserByID(ctx, assignedBy)
	if err != nil {
		s.log.Error("publishTaskAssigned: ошибка получения профиля назначившего",
			"error", err,
			"user_id", assignedBy,
		)
		assignedByName = "Пользователь"
	} else if assignedByProfile != nil {
		assignedByName = fmt.Sprintf("%s %s", assignedByProfile.FirstName, assignedByProfile.LastName)
		if assignedByName == " " {
			assignedByName = "Пользователь"
		}
	} else {
		assignedByName = "Пользователь"
	}

	// Определяем due date
	dueDate := time.Now().Add(24 * time.Hour) // по умолчанию +1 день
	if task.DueDate != nil {
		dueDate = *task.DueDate
	}

	// Публикуем событие
	if err := s.producer.PublishTaskAssigned(
		ctx,
		task.ID,
		task.EventID,
		*task.AssignedTo,
		task.Title,
		task.Description,
		event.Title,
		assignedEmail,
		assignedName,
		task.Priority,
		dueDate,
		assignedBy,
		assignedByName,
	); err != nil {
		s.log.Error("publishTaskAssigned: ошибка публикации", "error", err)
	} else {
		s.log.Info("publishTaskAssigned: событие опубликовано",
			"task_id", task.ID,
			"assigned_to", *task.AssignedTo,
			"assigned_name", assignedName,
		)
	}
}
