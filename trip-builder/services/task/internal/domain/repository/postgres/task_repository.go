package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type TaskRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewTaskRepository(db *sql.DB, schema string, log *logger.Logger) *TaskRepository {
	return &TaskRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *TaskRepository) table() string {
	return fmt.Sprintf("%s.tasks", r.schema)
}

func (r *TaskRepository) Create(ctx context.Context, task *models.Task) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (id, event_id, title, description, assigned_to, created_by, priority, status, due_date, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		task.ID, task.EventID, task.Title, task.Description, task.AssignedTo,
		task.CreatedBy, task.Priority, task.Status, task.DueDate, task.CreatedAt,
	)
	if err != nil {
		r.log.Error("TaskRepository.Create: ошибка создания", "error", err, "task_id", task.ID)
		return err
	}

	r.log.Debug("TaskRepository.Create: задача создана", "task_id", task.ID)
	return nil
}

func (r *TaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	query := fmt.Sprintf(`
		SELECT id, event_id, title, description, assigned_to, created_by, priority, status, due_date, completed_at, created_at, updated_at
		FROM %s WHERE id = $1
	`, r.table())

	task := &models.Task{}
	var assignedTo, description sql.NullString
	var dueDate, completedAt, updatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID, &task.EventID, &task.Title, &description, &assignedTo,
		&task.CreatedBy, &task.Priority, &task.Status, &dueDate, &completedAt,
		&task.CreatedAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("TaskRepository.GetByID: ошибка", "error", err, "task_id", id)
		return nil, err
	}

	if description.Valid {
		task.Description = description.String
	}
	if assignedTo.Valid {
		id, _ := uuid.Parse(assignedTo.String)
		task.AssignedTo = &id
	}
	if dueDate.Valid {
		task.DueDate = &dueDate.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	if updatedAt.Valid {
		task.UpdatedAt = &updatedAt.Time
	}

	return task, nil
}

func (r *TaskRepository) GetByEventID(ctx context.Context, eventID uuid.UUID) ([]models.Task, error) {
	query := fmt.Sprintf(`
		SELECT id, event_id, title, description, assigned_to, created_by, priority, status, due_date, completed_at, created_at, updated_at
		FROM %s WHERE event_id = $1
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, eventID)
	if err != nil {
		r.log.Error("TaskRepository.GetByEventID: ошибка запроса", "error", err, "event_id", eventID)
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var task models.Task
		var assignedTo, description sql.NullString
		var dueDate, completedAt, updatedAt sql.NullTime

		err := rows.Scan(
			&task.ID, &task.EventID, &task.Title, &description, &assignedTo,
			&task.CreatedBy, &task.Priority, &task.Status, &dueDate, &completedAt,
			&task.CreatedAt, &updatedAt,
		)
		if err != nil {
			r.log.Error("TaskRepository.GetByEventID: ошибка сканирования", "error", err)
			return nil, err
		}

		if description.Valid {
			task.Description = description.String
		}
		if assignedTo.Valid {
			id, _ := uuid.Parse(assignedTo.String)
			task.AssignedTo = &id
		}
		if dueDate.Valid {
			task.DueDate = &dueDate.Time
		}
		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}
		if updatedAt.Valid {
			task.UpdatedAt = &updatedAt.Time
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (r *TaskRepository) List(ctx context.Context, eventID *uuid.UUID, assignedTo *uuid.UUID, status string, limit, offset int) ([]models.Task, int64, error) {
	args := []interface{}{}
	argPos := 1
	conditions := []string{}

	if eventID != nil {
		conditions = append(conditions, fmt.Sprintf("event_id = $%d", argPos))
		args = append(args, *eventID)
		argPos++
	}

	if assignedTo != nil {
		conditions = append(conditions, fmt.Sprintf("assigned_to = $%d", argPos))
		args = append(args, *assignedTo)
		argPos++
	}

	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argPos))
		args = append(args, status)
		argPos++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			whereClause += " AND " + conditions[i]
		}
	}

	// Получаем общее количество
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", r.table(), whereClause)
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		r.log.Error("TaskRepository.List: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	// Получаем данные
	query := fmt.Sprintf(`
		SELECT id, event_id, title, description, assigned_to, created_by, priority, status, due_date, completed_at, created_at, updated_at
		FROM %s %s
		ORDER BY due_date ASC NULLS LAST, created_at DESC
		LIMIT $%d OFFSET $%d
	`, r.table(), whereClause, argPos, argPos+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		r.log.Error("TaskRepository.List: ошибка запроса", "error", err)
		return nil, 0, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var task models.Task
		var assignedToID, description sql.NullString
		var dueDate, completedAt, updatedAt sql.NullTime

		err := rows.Scan(
			&task.ID, &task.EventID, &task.Title, &description, &assignedToID,
			&task.CreatedBy, &task.Priority, &task.Status, &dueDate, &completedAt,
			&task.CreatedAt, &updatedAt,
		)
		if err != nil {
			r.log.Error("TaskRepository.List: ошибка сканирования", "error", err)
			return nil, 0, err
		}

		if description.Valid {
			task.Description = description.String
		}
		if assignedToID.Valid {
			id, _ := uuid.Parse(assignedToID.String)
			task.AssignedTo = &id
		}
		if dueDate.Valid {
			task.DueDate = &dueDate.Time
		}
		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}
		if updatedAt.Valid {
			task.UpdatedAt = &updatedAt.Time
		}

		tasks = append(tasks, task)
	}

	return tasks, total, nil
}

func (r *TaskRepository) Update(ctx context.Context, task *models.Task) error {
	query := fmt.Sprintf(`
		UPDATE %s SET
			title = $1,
			description = $2,
			assigned_to = $3,
			priority = $4,
			status = $5,
			due_date = $6,
			updated_at = NOW()
		WHERE id = $7
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		task.Title, task.Description, task.AssignedTo,
		task.Priority, task.Status, task.DueDate, task.ID,
	)
	if err != nil {
		r.log.Error("TaskRepository.Update: ошибка обновления", "error", err, "task_id", task.ID)
		return err
	}

	return nil
}

func (r *TaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.table())
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error("TaskRepository.Delete: ошибка удаления", "error", err, "task_id", id)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("TaskRepository.Delete: задача удалена", "task_id", id, "rows", rows)
	return nil
}

func (r *TaskRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, completedAt *time.Time) error {
	query := fmt.Sprintf(`
		UPDATE %s SET status = $1, completed_at = $2, updated_at = NOW()
		WHERE id = $3
	`, r.table())

	_, err := r.db.ExecContext(ctx, query, status, completedAt, id)
	if err != nil {
		r.log.Error("TaskRepository.UpdateStatus: ошибка", "error", err, "task_id", id, "status", status)
		return err
	}

	return nil
}

func (r *TaskRepository) CancelByEventID(ctx context.Context, eventID uuid.UUID) error {
	query := fmt.Sprintf(`
		UPDATE %s SET status = 'cancelled', updated_at = NOW()
		WHERE event_id = $1 AND status = 'pending'
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, eventID)
	if err != nil {
		r.log.Error("TaskRepository.CancelByEventID: ошибка", "error", err, "event_id", eventID)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("TaskRepository.CancelByEventID: задачи отменены", "event_id", eventID, "rows", rows)
	return nil
}
