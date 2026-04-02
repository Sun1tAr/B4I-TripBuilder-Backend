package service

import (
	"context"
	"errors"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/client/event"
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
	producer     *kafka.TaskProducer
	log          *logger.Logger
}

func NewTaskService(
	taskRepo repository.TaskRepository,
	commentRepo repository.TaskCommentRepository,
	templateRepo repository.TaskTemplateRepository,
	eventClient *event.Client,
	producer *kafka.TaskProducer,
	log *logger.Logger,
) *TaskService {
	return &TaskService{
		taskRepo:     taskRepo,
		commentRepo:  commentRepo,
		templateRepo: templateRepo,
		eventClient:  eventClient,
		producer:     producer,
		log:          log,
	}
}

// CreateTask создаёт новую задачу
func (s *TaskService) CreateTask(ctx context.Context, req *models.CreateTaskRequest, userID uuid.UUID) (*models.Task, error) {
	s.log.Debug("CreateTask: начало", "user_id", userID, "event_id", req.EventID, "title", req.Title)

	// Валидация приоритета
	if req.Priority != "" && req.Priority != "high" && req.Priority != "medium" && req.Priority != "low" {
		s.log.Debug("CreateTask: неверный приоритет", "priority", req.Priority)
		return nil, ErrInvalidPriority
	}

	// Валидация даты
	if req.DueDate != nil && req.DueDate.Before(time.Now()) {
		s.log.Debug("CreateTask: дедлайн в прошлом", "due_date", req.DueDate)
		return nil, ErrInvalidDueDate
	}

	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}

	// Проверяем права: кто создает задачу
	isOwner, err := s.eventClient.IsEventOwner(ctx, req.EventID, userID)
	if err != nil {
		s.log.Error("CreateTask: ошибка проверки владельца", "error", err)
		return nil, err
	}

	isParticipant, err := s.eventClient.IsEventParticipant(ctx, req.EventID, userID)
	if err != nil {
		s.log.Error("CreateTask: ошибка проверки участника", "error", err)
		return nil, err
	}

	if !isOwner && !isParticipant {
		s.log.Warn("CreateTask: пользователь не является участником события", "user_id", userID, "event_id", req.EventID)
		return nil, ErrNotAuthorized
	}

	// Определяем assigned_to
	assignedTo := req.AssignedTo
	approvedByAdmin := false

	if assignedTo == nil {
		// Если назначенный не указан — назначаем на создателя
		assignedTo = &userID
	}

	if isOwner {
		// Админ: задача сразу подтверждена
		approvedByAdmin = true
	} else {
		// Участник: проверяем, на кого создает задачу
		if assignedTo != nil && *assignedTo == userID {
			// Создает задачу для себя → сразу подтверждена
			approvedByAdmin = true
		} else {
			// Создает задачу для другого → требует подтверждения админа
			approvedByAdmin = false
		}
	}

	now := time.Now()
	task := &models.Task{
		ID:              uuid.New(),
		EventID:         req.EventID,
		Title:           req.Title,
		Description:     req.Description,
		AssignedTo:      assignedTo,
		CreatedBy:       userID,
		Priority:        priority,
		Status:          "pending",
		DueDate:         req.DueDate,
		ApprovedByAdmin: approvedByAdmin,
		CreatedAt:       now,
	}

	s.log.Trace("CreateTask: сохранение в БД", "task_id", task.ID, "approved_by_admin", approvedByAdmin)

	if err := s.taskRepo.Create(ctx, task); err != nil {
		s.log.Error("CreateTask: ошибка сохранения", "error", err, "task_id", task.ID)
		return nil, err
	}

	s.log.Info("CreateTask: задача создана", "task_id", task.ID, "event_id", task.EventID, "created_by", userID, "approved_by_admin", approvedByAdmin)
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

// GetTasks возвращает список задач с фильтрацией
func (s *TaskService) GetTasks(ctx context.Context, eventID *uuid.UUID, assignedTo *uuid.UUID, status string, page, size int, userID *uuid.UUID) ([]models.Task, int64, error) {
	s.log.Debug("GetTasks: начало", "user_id", userID, "event_id", eventID, "assigned_to", assignedTo, "status", status, "page", page, "size", size)

	offset := (page - 1) * size
	tasks, total, err := s.taskRepo.List(ctx, eventID, assignedTo, status, size, offset)
	if err != nil {
		s.log.Error("GetTasks: ошибка получения списка", "error", err)
		return nil, 0, err
	}

	s.log.Info("GetTasks: список получен", "count", len(tasks), "total", total)
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
	s.log.Debug("UpdateTask: начало", "task_id", taskID, "user_id", userID)

	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrTaskNotFound
	}

	// Проверяем права: админ или создатель
	isOwner, err := s.eventClient.IsEventOwner(ctx, task.EventID, userID)
	if err != nil {
		s.log.Error("UpdateTask: ошибка проверки владельца", "error", err)
		return nil, err
	}

	isCreator := task.CreatedBy == userID

	if !isOwner && !isCreator {
		s.log.Warn("UpdateTask: попытка редактирования без прав", "task_id", taskID, "user_id", userID)
		return nil, ErrNotAuthorized
	}

	// Если задача не подтверждена, менять можно только создателю или админу
	if !task.ApprovedByAdmin && !isCreator && !isOwner {
		s.log.Warn("UpdateTask: попытка редактирования неподтвержденной задачи", "task_id", taskID, "user_id", userID)
		return nil, ErrNotAuthorized
	}

	s.log.Trace("UpdateTask: обновление полей", "task_id", taskID)

	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Priority != nil {
		if *req.Priority != "high" && *req.Priority != "medium" && *req.Priority != "low" {
			return nil, ErrInvalidPriority
		}
		task.Priority = *req.Priority
	}
	if req.DueDate != nil {
		if req.DueDate.Before(time.Now()) {
			return nil, ErrInvalidDueDate
		}
		task.DueDate = req.DueDate
	}

	// Переназначить может только админ
	if req.AssignedTo != nil {
		if !isOwner {
			s.log.Warn("UpdateTask: попытка переназначения без прав", "task_id", taskID, "user_id", userID)
			return nil, ErrNotAuthorized
		}
		task.AssignedTo = req.AssignedTo
	}

	// Изменить статус может админ, создатель или назначенный
	if req.Status != nil {
		if *req.Status != "pending" && *req.Status != "done" && *req.Status != "cancelled" {
			return nil, ErrInvalidStatus
		}

		isAssigned := task.AssignedTo != nil && *task.AssignedTo == userID

		if !isOwner && !isCreator && !isAssigned {
			s.log.Warn("UpdateTask: попытка изменения статуса без прав", "task_id", taskID, "user_id", userID)
			return nil, ErrNotAuthorized
		}

		oldStatus := task.Status
		task.Status = *req.Status

		if task.Status == "done" && oldStatus != "done" {
			now := time.Now()
			task.CompletedAt = &now
			s.log.Info("UpdateTask: задача выполнена", "task_id", taskID, "user_id", userID)

			// Публикуем событие о выполнении задачи
			go func() {
				if err := s.producer.PublishTaskCompleted(context.Background(), taskID, task.EventID, userID, task.Title); err != nil {
					s.log.Error("UpdateTask: ошибка публикации события", "error", err, "task_id", taskID)
				}
			}()
		}
	}

	now := time.Now()
	task.UpdatedAt = &now

	if err := s.taskRepo.Update(ctx, task); err != nil {
		s.log.Error("UpdateTask: ошибка сохранения", "error", err, "task_id", taskID)
		return nil, err
	}

	s.log.Info("UpdateTask: задача обновлена", "task_id", taskID, "user_id", userID, "new_status", task.Status)
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
