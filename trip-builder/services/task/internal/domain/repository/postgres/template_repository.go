package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/task/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type TaskTemplateRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewTaskTemplateRepository(db *sql.DB, schema string, log *logger.Logger) *TaskTemplateRepository {
	return &TaskTemplateRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *TaskTemplateRepository) table() string {
	return fmt.Sprintf("%s.task_templates", r.schema)
}

func (r *TaskTemplateRepository) List(ctx context.Context) ([]models.TaskTemplate, error) {
	query := fmt.Sprintf(`
		SELECT id, title, description, trigger_days, priority, created_at
		FROM %s
		ORDER BY trigger_days ASC
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		r.log.Error("TaskTemplateRepository.List: ошибка запроса", "error", err)
		return nil, err
	}
	defer rows.Close()

	var templates []models.TaskTemplate
	for rows.Next() {
		var t models.TaskTemplate
		var description sql.NullString

		err := rows.Scan(&t.ID, &t.Title, &description, &t.TriggerDays, &t.Priority, &t.CreatedAt)
		if err != nil {
			r.log.Error("TaskTemplateRepository.List: ошибка сканирования", "error", err)
			return nil, err
		}

		if description.Valid {
			t.Description = description.String
		}
		templates = append(templates, t)
	}

	return templates, nil
}

func (r *TaskTemplateRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.TaskTemplate, error) {
	query := fmt.Sprintf(`
		SELECT id, title, description, trigger_days, priority, created_at
		FROM %s WHERE id = $1
	`, r.table())

	t := &models.TaskTemplate{}
	var description sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.Title, &description, &t.TriggerDays, &t.Priority, &t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("TaskTemplateRepository.GetByID: ошибка", "error", err, "template_id", id)
		return nil, err
	}

	if description.Valid {
		t.Description = description.String
	}

	return t, nil
}
