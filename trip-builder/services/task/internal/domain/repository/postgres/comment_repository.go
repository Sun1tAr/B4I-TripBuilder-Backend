package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type TaskCommentRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewTaskCommentRepository(db *sql.DB, schema string, log *logger.Logger) *TaskCommentRepository {
	return &TaskCommentRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *TaskCommentRepository) table() string {
	return fmt.Sprintf("%s.task_comments", r.schema)
}

func (r *TaskCommentRepository) Create(ctx context.Context, comment *models.TaskComment) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (id, task_id, user_id, text, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		comment.ID, comment.TaskID, comment.UserID, comment.Text, comment.CreatedAt,
	)
	if err != nil {
		r.log.Error("TaskCommentRepository.Create: ошибка создания", "error", err, "comment_id", comment.ID)
		return err
	}

	return nil
}

func (r *TaskCommentRepository) GetByTaskID(ctx context.Context, taskID uuid.UUID, limit, offset int) ([]models.TaskComment, int64, error) {
	query := fmt.Sprintf(`
		SELECT id, task_id, user_id, text, created_at
		FROM %s WHERE task_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, taskID, limit, offset)
	if err != nil {
		r.log.Error("TaskCommentRepository.GetByTaskID: ошибка запроса", "error", err, "task_id", taskID)
		return nil, 0, err
	}
	defer rows.Close()

	var comments []models.TaskComment
	for rows.Next() {
		var comment models.TaskComment
		err := rows.Scan(&comment.ID, &comment.TaskID, &comment.UserID, &comment.Text, &comment.CreatedAt)
		if err != nil {
			r.log.Error("TaskCommentRepository.GetByTaskID: ошибка сканирования", "error", err)
			return nil, 0, err
		}
		comments = append(comments, comment)
	}

	// Получаем общее количество
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE task_id = $1", r.table())
	var total int64
	err = r.db.QueryRowContext(ctx, countQuery, taskID).Scan(&total)
	if err != nil {
		r.log.Error("TaskCommentRepository.GetByTaskID: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	return comments, total, nil
}

func (r *TaskCommentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.table())
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error("TaskCommentRepository.Delete: ошибка удаления", "error", err, "comment_id", id)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("TaskCommentRepository.Delete: комментарий удалён", "comment_id", id, "rows", rows)
	return nil
}

func (r *TaskCommentRepository) IsAuthor(ctx context.Context, commentID, userID uuid.UUID) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1 AND user_id = $2)", r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, commentID, userID).Scan(&exists)
	if err != nil {
		r.log.Error("TaskCommentRepository.IsAuthor: ошибка проверки", "error", err, "comment_id", commentID, "user_id", userID)
		return false, err
	}
	return exists, nil
}
