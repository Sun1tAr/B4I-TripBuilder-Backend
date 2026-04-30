package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/service"
	commondto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/common"
	taskdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/task"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type TaskHandler struct {
	taskService *service.TaskService
	log         *logger.Logger
}

func NewTaskHandler(taskService *service.TaskService, log *logger.Logger) *TaskHandler {
	return &TaskHandler{
		taskService: taskService,
		log:         log,
	}
}

func (h *TaskHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(commondto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func (h *TaskHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *TaskHandler) getUserIDFromContext(r *http.Request) *uuid.UUID {
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

func taskToResponse(task *models.Task) *taskdto.TaskResponse {
	resp := &taskdto.TaskResponse{
		ID:              task.ID,
		EventID:         task.EventID,
		Title:           task.Title,
		Description:     task.Description,
		CreatedBy:       task.CreatedBy,
		Priority:        task.Priority,
		Status:          task.Status,
		DueDate:         task.DueDate,
		CompletedAt:     task.CompletedAt,
		ApprovedByAdmin: task.ApprovedByAdmin,
		CreatedAt:       task.CreatedAt,
		UpdatedAt:       task.UpdatedAt,
	}
	if task.AssignedTo != nil {
		resp.AssignedTo = task.AssignedTo
	}
	return resp
}

// GetComments возвращает комментарии к задаче
func (h *TaskHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	taskIDStr := vars["id"]

	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		log.Debug("GetComments: неверный ID задачи", "task_id", taskIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid task id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	// Проверяем, что пользователь имеет доступ к задаче (участник события)
	task, err := h.taskService.GetTaskByID(r.Context(), taskID, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			log.Debug("GetComments: задача не найдена", "task_id", taskID)
			h.writeError(w, http.StatusNotFound, "task not found", "")
		default:
			log.Error("Ошибка получения задачи", "error", err, "task_id", taskID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}
	if task == nil {
		h.writeError(w, http.StatusNotFound, "task not found", "")
		return
	}

	// Парсим пагинацию
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 || size > 1000 {
		size = 1000
	}

	comments, total, err := h.taskService.GetComments(r.Context(), taskID, page, size)
	if err != nil {
		log.Error("Ошибка получения комментариев", "error", err, "task_id", taskID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	items := make([]taskdto.TaskCommentResponse, len(comments))
	for i, c := range comments {
		items[i] = taskdto.TaskCommentResponse{
			ID:        c.ID,
			UserID:    c.UserID,
			Text:      c.Text,
			CreatedAt: c.CreatedAt,
		}
	}

	totalPages := (int(total) + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}

	response := map[string]interface{}{
		"items":       items,
		"total":       total,
		"page":        page,
		"size":        size,
		"total_pages": totalPages,
	}

	log.Info("GetComments: комментарии получены", "task_id", taskID, "user_id", userID, "count", len(items))
	h.writeJSON(w, http.StatusOK, response)
}

// GetTasks возвращает список задач
func (h *TaskHandler) GetTasks(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("GetTasks: запрос списка задач")

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	// Парсим query параметры
	eventIDStr := r.URL.Query().Get("event_id")
	var eventID *uuid.UUID
	if eventIDStr != "" {
		id, err := uuid.Parse(eventIDStr)
		if err == nil {
			eventID = &id
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 || size > 10000 {
		size = 10000
	}

	log.Debug("GetTasks: параметры", "event_id", eventID, "page", page, "size", size)

	tasks, total, err := h.taskService.GetTasks(r.Context(), eventID, *userID, page, size)
	if err != nil {
		log.Error("Ошибка получения задач", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	items := make([]taskdto.TaskResponse, 0, len(tasks))
	for _, task := range tasks {
		commentsCount, _ := h.taskService.GetCommentsCount(r.Context(), task.ID)

		item := taskdto.TaskResponse{
			ID:              task.ID,
			EventID:         task.EventID,
			Title:           task.Title,
			Description:     task.Description,
			CreatedBy:       task.CreatedBy,
			Priority:        task.Priority,
			Status:          task.Status,
			DueDate:         task.DueDate,
			CompletedAt:     task.CompletedAt,
			ApprovedByAdmin: task.ApprovedByAdmin,
			CommentsCount:   commentsCount,
			CreatedAt:       task.CreatedAt,
			UpdatedAt:       task.UpdatedAt,
		}
		if task.AssignedTo != nil {
			item.AssignedTo = task.AssignedTo
		}
		items = append(items, item)
	}

	totalPages := (int(total) + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}

	response := taskdto.ListTasksResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		Size:       size,
		TotalPages: totalPages,
	}

	log.Info("GetTasks: список задач получен", "user_id", userID, "count", len(items), "total", total)
	h.writeJSON(w, http.StatusOK, response)
}

// GetTaskByID возвращает задачу с комментариями
func (h *TaskHandler) GetTaskByID(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	taskIDStr := vars["id"]

	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		log.Debug("GetTaskByID: неверный ID", "task_id", taskIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid task id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	log.Debug("GetTaskByID: запрос", "task_id", taskID, "user_id", userID)

	task, err := h.taskService.GetTaskByID(r.Context(), taskID, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			log.Debug("GetTaskByID: задача не найдена", "task_id", taskID)
			h.writeError(w, http.StatusNotFound, "task not found", "")
		default:
			log.Error("Ошибка получения задачи", "error", err, "task_id", taskID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	// Получаем комментарии
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 || size > 100 {
		size = 20
	}

	comments, total, err := h.taskService.GetComments(r.Context(), taskID, page, size)
	if err != nil {
		log.Error("Ошибка получения комментариев", "error", err, "task_id", taskID)
		comments = []models.TaskComment{}
	}

	commentItems := make([]taskdto.TaskCommentResponse, len(comments))
	for i, c := range comments {
		commentItems[i] = taskdto.TaskCommentResponse{
			ID:        c.ID,
			UserID:    c.UserID,
			Text:      c.Text,
			CreatedAt: c.CreatedAt,
		}
	}

	response := taskdto.TaskDetailResponse{
		TaskResponse: *taskToResponse(task),
		Comments:     commentItems,
	}
	_ = total // не используем в ответе

	log.Info("GetTaskByID: задача получена", "task_id", taskID, "user_id", userID)
	h.writeJSON(w, http.StatusOK, response)
}

// CreateTask создаёт новую задачу
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Info("CreateTask: создание задачи")

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req taskdto.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("CreateTask: ошибка декодирования", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	log.Debug("CreateTask: данные запроса", "event_id", req.EventID, "title", req.Title, "priority", req.Priority)

	createReq := &models.CreateTaskRequest{
		EventID:     req.EventID,
		Title:       req.Title,
		Description: req.Description,
		AssignedTo:  req.AssignedTo,
		Priority:    req.Priority,
		DueDate:     req.DueDate,
	}

	task, err := h.taskService.CreateTask(r.Context(), createReq, *userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPriority):
			h.writeError(w, http.StatusBadRequest, "invalid priority", "")
		case errors.Is(err, service.ErrInvalidDueDate):
			h.writeError(w, http.StatusBadRequest, "due date cannot be in the past", "")
		default:
			log.Error("Ошибка создания задачи", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("CreateTask: задача создана", "task_id", task.ID, "user_id", userID)
	h.writeJSON(w, http.StatusCreated, taskToResponse(task))
}

// UpdateTask обновляет задачу
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	taskIDStr := vars["id"]

	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		log.Debug("UpdateTask: неверный ID", "task_id", taskIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid task id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req taskdto.UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("UpdateTask: ошибка декодирования", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	log.Debug("UpdateTask: данные запроса", "task_id", taskID, "user_id", userID, "status", req.Status)

	updateReq := &models.UpdateTaskRequest{
		Title:       req.Title,
		Description: req.Description,
		AssignedTo:  req.AssignedTo,
		Priority:    req.Priority,
		Status:      req.Status,
		DueDate:     req.DueDate,
	}

	task, err := h.taskService.UpdateTask(r.Context(), taskID, updateReq, *userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			h.writeError(w, http.StatusNotFound, "task not found", "")
		case errors.Is(err, service.ErrNotAuthorized):
			h.writeError(w, http.StatusForbidden, "not authorized to modify this task", "")
		case errors.Is(err, service.ErrInvalidPriority):
			h.writeError(w, http.StatusBadRequest, "invalid priority", "")
		case errors.Is(err, service.ErrInvalidStatus):
			h.writeError(w, http.StatusBadRequest, "invalid status", "")
		case errors.Is(err, service.ErrInvalidDueDate):
			h.writeError(w, http.StatusBadRequest, "due date cannot be in the past", "")
		default:
			log.Error("Ошибка обновления задачи", "error", err, "task_id", taskID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("UpdateTask: задача обновлена", "task_id", taskID, "user_id", userID, "new_status", task.Status)
	h.writeJSON(w, http.StatusOK, taskToResponse(task))
}

// DeleteTask удаляет задачу
func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	taskIDStr := vars["id"]

	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		log.Debug("DeleteTask: неверный ID", "task_id", taskIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid task id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	log.Debug("DeleteTask: удаление задачи", "task_id", taskID, "user_id", userID)

	err = h.taskService.DeleteTask(r.Context(), taskID, *userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			h.writeError(w, http.StatusNotFound, "task not found", "")
		case errors.Is(err, service.ErrNotAuthorized):
			h.writeError(w, http.StatusForbidden, "not authorized to delete this task", "")
		default:
			log.Error("Ошибка удаления задачи", "error", err, "task_id", taskID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("DeleteTask: задача удалена", "task_id", taskID, "user_id", userID)
	h.writeJSON(w, http.StatusOK, taskdto.MessageResponse{Message: "task deleted successfully"})
}

// AddComment добавляет комментарий к задаче
func (h *TaskHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	taskIDStr := vars["id"]

	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		log.Debug("AddComment: неверный ID задачи", "task_id", taskIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid task id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req taskdto.AddCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("AddComment: ошибка декодирования", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	log.Debug("AddComment: добавление комментария", "task_id", taskID, "user_id", userID)

	comment, err := h.taskService.AddComment(r.Context(), taskID, *userID, req.Text)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			h.writeError(w, http.StatusNotFound, "task not found", "")
		default:
			log.Error("Ошибка добавления комментария", "error", err, "task_id", taskID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("AddComment: комментарий добавлен", "comment_id", comment.ID, "task_id", taskID, "user_id", userID)

	response := taskdto.TaskCommentResponse{
		ID:        comment.ID,
		UserID:    comment.UserID,
		Text:      comment.Text,
		CreatedAt: comment.CreatedAt,
	}
	h.writeJSON(w, http.StatusCreated, response)
}

// GetTemplates возвращает список шаблонов задач
func (h *TaskHandler) GetTemplates(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("GetTemplates: запрос шаблонов")

	templates, err := h.taskService.GetTemplates(r.Context())
	if err != nil {
		log.Error("Ошибка получения шаблонов", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	response := make([]taskdto.TaskTemplateResponse, len(templates))
	for i, t := range templates {
		response[i] = taskdto.TaskTemplateResponse{
			ID:          t.ID,
			Title:       t.Title,
			Description: t.Description,
			TriggerDays: t.TriggerDays,
			Priority:    t.Priority,
		}
	}

	log.Info("GetTemplates: шаблоны получены", "count", len(response))
	h.writeJSON(w, http.StatusOK, response)
}

// ApproveTask подтверждает задачу (только для админа)
func (h *TaskHandler) ApproveTask(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	taskIDStr := vars["id"]

	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		log.Debug("ApproveTask: неверный ID", "task_id", taskIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid task id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	log.Debug("ApproveTask: подтверждение задачи", "task_id", taskID, "user_id", userID)

	task, err := h.taskService.ApproveTask(r.Context(), taskID, *userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			log.Debug("ApproveTask: задача не найдена", "task_id", taskID)
			h.writeError(w, http.StatusNotFound, "task not found", "")
		case errors.Is(err, service.ErrNotAuthorized):
			log.Debug("ApproveTask: нет прав", "task_id", taskID, "user_id", userID)
			h.writeError(w, http.StatusForbidden, "not authorized", "")
		case errors.Is(err, service.ErrAlreadyApproved):
			log.Debug("ApproveTask: задача уже подтверждена", "task_id", taskID)
			h.writeError(w, http.StatusConflict, "task already approved", "")
		default:
			log.Error("Ошибка подтверждения задачи", "error", err, "task_id", taskID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	log.Info("ApproveTask: задача подтверждена", "task_id", taskID, "user_id", userID)
	h.writeJSON(w, http.StatusOK, taskToResponse(task))
}
