package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type TemplateRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewTemplateRepository(db *sql.DB, schema string, log *logger.Logger) *TemplateRepository {
	return &TemplateRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *TemplateRepository) table() string {
	return fmt.Sprintf("%s.email_templates", r.schema)
}

func (r *TemplateRepository) GetByType(ctx context.Context, templateType, language string) (*models.EmailTemplate, error) {
	r.log.Debug("TemplateRepository.GetByType: начало",
		"type", templateType,
		"language", language,
	)

	// Сначала ищем точное совпадение
	query := fmt.Sprintf(`
		SELECT id, type, subject, body_template, language, created_at, updated_at
		FROM %s 
		WHERE type = $1 AND language = $2
	`, r.table())

	template := &models.EmailTemplate{}
	var updatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, templateType, language).Scan(
		&template.ID,
		&template.Type,
		&template.Subject,
		&template.BodyTemplate,
		&template.Language,
		&template.CreatedAt,
		&updatedAt,
	)

	if err == nil {
		if updatedAt.Valid {
			template.UpdatedAt = &updatedAt.Time
		}
		r.log.Debug("TemplateRepository.GetByType: шаблон найден",
			"type", templateType,
			"language", language,
		)
		return template, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		r.log.Error("TemplateRepository.GetByType: ошибка запроса", "error", err)
		return nil, err
	}

	// Если не нашли с указанным языком, ищем с дефолтным языком
	r.log.Debug("TemplateRepository.GetByType: точное совпадение не найдено, ищем fallback",
		"type", templateType,
	)

	query = fmt.Sprintf(`
		SELECT id, type, subject, body_template, language, created_at, updated_at
		FROM %s 
		WHERE type = $1 AND language = 'ru'
	`, r.table())

	err = r.db.QueryRowContext(ctx, query, templateType).Scan(
		&template.ID,
		&template.Type,
		&template.Subject,
		&template.BodyTemplate,
		&template.Language,
		&template.CreatedAt,
		&updatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Warn("TemplateRepository.GetByType: шаблон не найден даже с fallback",
				"type", templateType,
			)
			return nil, nil
		}
		r.log.Error("TemplateRepository.GetByType: ошибка запроса fallback", "error", err)
		return nil, err
	}

	if updatedAt.Valid {
		template.UpdatedAt = &updatedAt.Time
	}

	r.log.Debug("TemplateRepository.GetByType: найден fallback шаблон",
		"type", templateType,
		"fallback_language", template.Language,
	)
	return template, nil
}

func (r *TemplateRepository) GetAllByType(ctx context.Context, templateType string) ([]*models.EmailTemplate, error) {
	r.log.Debug("TemplateRepository.GetAllByType: начало", "type", templateType)

	query := fmt.Sprintf(`
		SELECT id, type, subject, body_template, language, created_at, updated_at
		FROM %s 
		WHERE type = $1
		ORDER BY language
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, templateType)
	if err != nil {
		r.log.Error("TemplateRepository.GetAllByType: ошибка запроса", "error", err)
		return nil, err
	}
	defer rows.Close()

	var templates []*models.EmailTemplate
	for rows.Next() {
		template := &models.EmailTemplate{}
		var updatedAt sql.NullTime

		err := rows.Scan(
			&template.ID,
			&template.Type,
			&template.Subject,
			&template.BodyTemplate,
			&template.Language,
			&template.CreatedAt,
			&updatedAt,
		)
		if err != nil {
			r.log.Error("TemplateRepository.GetAllByType: ошибка сканирования", "error", err)
			continue
		}

		if updatedAt.Valid {
			template.UpdatedAt = &updatedAt.Time
		}
		templates = append(templates, template)
	}

	r.log.Debug("TemplateRepository.GetAllByType: шаблоны получены",
		"type", templateType,
		"count", len(templates),
	)
	return templates, nil
}

func (r *TemplateRepository) Upsert(ctx context.Context, template *models.EmailTemplate) error {
	r.log.Debug("TemplateRepository.Upsert: начало",
		"type", template.Type,
		"language", template.Language,
	)

	now := time.Now()

	query := fmt.Sprintf(`
		INSERT INTO %s (id, type, subject, body_template, language, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (type, language) DO UPDATE SET
			subject = EXCLUDED.subject,
			body_template = EXCLUDED.body_template,
			updated_at = EXCLUDED.updated_at
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		template.ID,
		template.Type,
		template.Subject,
		template.BodyTemplate,
		template.Language,
		template.CreatedAt,
		&now,
	)
	if err != nil {
		r.log.Error("TemplateRepository.Upsert: ошибка", "error", err)
		return err
	}

	r.log.Info("TemplateRepository.Upsert: шаблон сохранен",
		"type", template.Type,
		"language", template.Language,
	)
	return nil
}
