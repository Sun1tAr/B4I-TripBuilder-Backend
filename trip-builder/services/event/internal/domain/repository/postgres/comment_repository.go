package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type CommentRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewCommentRepository(db *sql.DB, schema string, log *logger.Logger) *CommentRepository {
	return &CommentRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *CommentRepository) table() string {
	return fmt.Sprintf("%s.waypoint_comments", r.schema)
}

func (r *CommentRepository) Create(ctx context.Context, comment *models.Comment) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (id, event_id, waypoint_id, user_id, text, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		comment.ID, comment.EventID, comment.WaypointID, comment.UserID,
		comment.Text, comment.CreatedAt,
	)
	if err != nil {
		r.log.Error("CommentRepository.Create: ошибка создания", "error", err, "comment_id", comment.ID)
		return err
	}

	r.log.Debug("CommentRepository.Create: комментарий создан", "comment_id", comment.ID)
	return nil
}

func (r *CommentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Comment, error) {
	query := fmt.Sprintf(`
		SELECT id, event_id, waypoint_id, user_id, text, created_at, updated_at
		FROM %s WHERE id = $1
	`, r.table())

	comment := &models.Comment{}
	var updatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&comment.ID, &comment.EventID, &comment.WaypointID, &comment.UserID,
		&comment.Text, &comment.CreatedAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("CommentRepository.GetByID: ошибка", "error", err, "comment_id", id)
		return nil, err
	}

	if updatedAt.Valid {
		comment.UpdatedAt = &updatedAt.Time
	}

	return comment, nil
}

func (r *CommentRepository) GetByWaypointID(ctx context.Context, waypointID uuid.UUID, limit, offset int) ([]models.Comment, int64, error) {
	query := fmt.Sprintf(`
		SELECT id, event_id, waypoint_id, user_id, text, created_at, updated_at
		FROM %s WHERE waypoint_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, waypointID, limit, offset)
	if err != nil {
		r.log.Error("CommentRepository.GetByWaypointID: ошибка запроса", "error", err, "waypoint_id", waypointID)
		return nil, 0, err
	}
	defer rows.Close()

	var comments []models.Comment
	for rows.Next() {
		var comment models.Comment
		var updatedAt sql.NullTime

		err := rows.Scan(
			&comment.ID, &comment.EventID, &comment.WaypointID, &comment.UserID,
			&comment.Text, &comment.CreatedAt, &updatedAt,
		)
		if err != nil {
			r.log.Error("CommentRepository.GetByWaypointID: ошибка сканирования", "error", err)
			return nil, 0, err
		}

		if updatedAt.Valid {
			comment.UpdatedAt = &updatedAt.Time
		}
		comments = append(comments, comment)
	}

	// Получаем общее количество
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE waypoint_id = $1", r.table())
	var total int64
	err = r.db.QueryRowContext(ctx, countQuery, waypointID).Scan(&total)
	if err != nil {
		r.log.Error("CommentRepository.GetByWaypointID: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	return comments, total, nil
}

func (r *CommentRepository) Update(ctx context.Context, comment *models.Comment) error {
	query := fmt.Sprintf(`
		UPDATE %s SET text = $1, updated_at = NOW()
		WHERE id = $2
	`, r.table())

	_, err := r.db.ExecContext(ctx, query, comment.Text, comment.ID)
	if err != nil {
		r.log.Error("CommentRepository.Update: ошибка обновления", "error", err, "comment_id", comment.ID)
		return err
	}

	return nil
}

func (r *CommentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.table())
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error("CommentRepository.Delete: ошибка удаления", "error", err, "comment_id", id)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("CommentRepository.Delete: комментарий удалён", "comment_id", id, "rows", rows)
	return nil
}

func (r *CommentRepository) DeleteByWaypointID(ctx context.Context, waypointID uuid.UUID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE waypoint_id = $1", r.table())
	result, err := r.db.ExecContext(ctx, query, waypointID)
	if err != nil {
		r.log.Error("CommentRepository.DeleteByWaypointID: ошибка удаления", "error", err, "waypoint_id", waypointID)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("CommentRepository.DeleteByWaypointID: комментарии удалены", "waypoint_id", waypointID, "rows", rows)
	return nil
}

func (r *CommentRepository) IsAuthor(ctx context.Context, commentID, userID uuid.UUID) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1 AND user_id = $2)", r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, commentID, userID).Scan(&exists)
	if err != nil {
		r.log.Error("CommentRepository.IsAuthor: ошибка проверки", "error", err, "comment_id", commentID, "user_id", userID)
		return false, err
	}
	return exists, nil
}
