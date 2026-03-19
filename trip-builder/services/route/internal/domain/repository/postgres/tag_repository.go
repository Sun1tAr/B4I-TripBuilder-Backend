package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type TagRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewTagRepository(db *sql.DB, schema string, log *logger.Logger) *TagRepository {
	return &TagRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *TagRepository) table() string {
	return fmt.Sprintf("%s.tags", r.schema)
}

func (r *TagRepository) routeTagsTable() string {
	return fmt.Sprintf("%s.route_tags", r.schema)
}

func (r *TagRepository) Create(ctx context.Context, name string) (*models.Tag, error) {
	query := fmt.Sprintf(`
		INSERT INTO %s (id, name, routes_count, created_at)
		VALUES (gen_random_uuid(), $1, 0, NOW())
		RETURNING id, name, routes_count, created_at
	`, r.table())

	tag := &models.Tag{}
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&tag.ID, &tag.Name, &tag.RoutesCount, &tag.CreatedAt,
	)
	if err != nil {
		r.log.Error("TagRepository.Create: ошибка создания тега", "error", err, "name", name)
		return nil, err
	}

	r.log.Debug("TagRepository.Create: тег создан", "tag_id", tag.ID, "name", name)
	return tag, nil
}

func (r *TagRepository) GetByName(ctx context.Context, name string) (*models.Tag, error) {
	query := fmt.Sprintf(`
		SELECT id, name, routes_count, created_at
		FROM %s WHERE name = $1
	`, r.table())

	tag := &models.Tag{}
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&tag.ID, &tag.Name, &tag.RoutesCount, &tag.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("TagRepository.GetByName: ошибка получения тега", "error", err, "name", name)
		return nil, err
	}

	return tag, nil
}

func (r *TagRepository) GetOrCreate(ctx context.Context, name string) (*models.Tag, error) {
	// Пробуем получить существующий тег
	tag, err := r.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if tag != nil {
		return tag, nil
	}

	// Создаём новый
	return r.Create(ctx, name)
}

func (r *TagRepository) GetAll(ctx context.Context, limit, offset int) ([]models.Tag, int64, error) {
	query := fmt.Sprintf(`
		SELECT id, name, routes_count, created_at
		FROM %s
		ORDER BY routes_count DESC, name
		LIMIT $1 OFFSET $2
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		r.log.Error("TagRepository.GetAll: ошибка запроса", "error", err)
		return nil, 0, err
	}
	defer rows.Close()

	var tags []models.Tag
	for rows.Next() {
		var tag models.Tag
		err := rows.Scan(&tag.ID, &tag.Name, &tag.RoutesCount, &tag.CreatedAt)
		if err != nil {
			r.log.Error("TagRepository.GetAll: ошибка сканирования", "error", err)
			return nil, 0, err
		}
		tags = append(tags, tag)
	}

	// Получаем общее количество
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.table())
	var total int64
	err = r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		r.log.Error("TagRepository.GetAll: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	return tags, total, nil
}

func (r *TagRepository) GetByRouteID(ctx context.Context, routeID uuid.UUID) ([]models.Tag, error) {
	query := fmt.Sprintf(`
		SELECT t.id, t.name, t.routes_count, t.created_at
		FROM %s t
		JOIN %s rt ON t.id = rt.tag_id
		WHERE rt.route_id = $1
		ORDER BY t.name
	`, r.table(), r.routeTagsTable())

	rows, err := r.db.QueryContext(ctx, query, routeID)
	if err != nil {
		r.log.Error("TagRepository.GetByRouteID: ошибка запроса", "error", err, "route_id", routeID)
		return nil, err
	}
	defer rows.Close()

	var tags []models.Tag
	for rows.Next() {
		var tag models.Tag
		err := rows.Scan(&tag.ID, &tag.Name, &tag.RoutesCount, &tag.CreatedAt)
		if err != nil {
			r.log.Error("TagRepository.GetByRouteID: ошибка сканирования", "error", err)
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

func (r *TagRepository) AttachToRoute(ctx context.Context, routeID uuid.UUID, tagIDs []uuid.UUID) error {
	if len(tagIDs) == 0 {
		return nil
	}

	// Строим запрос с несколькими значениями
	valueStrings := make([]string, 0, len(tagIDs))
	args := make([]interface{}, 0, len(tagIDs)*2)

	for i, tagID := range tagIDs {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, NOW())", i*2+1, i*2+2))
		args = append(args, routeID, tagID)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (route_id, tag_id, created_at)
		VALUES %s
		ON CONFLICT (route_id, tag_id) DO NOTHING
	`, r.routeTagsTable(), strings.Join(valueStrings, ","))

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.log.Error("TagRepository.AttachToRoute: ошибка привязки тегов", "error", err, "route_id", routeID)
		return err
	}

	// Увеличиваем счётчик для каждого тега
	for _, tagID := range tagIDs {
		if err := r.IncrementCount(ctx, tagID); err != nil {
			r.log.Error("TagRepository.AttachToRoute: ошибка инкремента счётчика", "error", err, "tag_id", tagID)
			// Не фатально, продолжаем
		}
	}

	r.log.Debug("TagRepository.AttachToRoute: теги привязаны", "route_id", routeID, "count", len(tagIDs))
	return nil
}

func (r *TagRepository) ReplaceForRoute(ctx context.Context, routeID uuid.UUID, tagIDs []uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("TagRepository.ReplaceForRoute: ошибка начала транзакции", "error", err)
		return err
	}
	defer tx.Rollback()

	// Получаем старые теги для декремента
	oldTagsQuery := fmt.Sprintf("SELECT tag_id FROM %s WHERE route_id = $1", r.routeTagsTable())
	rows, err := tx.QueryContext(ctx, oldTagsQuery, routeID)
	if err != nil {
		r.log.Error("TagRepository.ReplaceForRoute: ошибка получения старых тегов", "error", err)
		return err
	}

	var oldTagIDs []uuid.UUID
	for rows.Next() {
		var tagID uuid.UUID
		if err := rows.Scan(&tagID); err != nil {
			rows.Close()
			r.log.Error("TagRepository.ReplaceForRoute: ошибка сканирования", "error", err)
			return err
		}
		oldTagIDs = append(oldTagIDs, tagID)
	}
	rows.Close()

	// Удаляем старые связи
	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE route_id = $1", r.routeTagsTable())
	_, err = tx.ExecContext(ctx, deleteQuery, routeID)
	if err != nil {
		r.log.Error("TagRepository.ReplaceForRoute: ошибка удаления старых связей", "error", err)
		return err
	}

	// Добавляем новые
	if len(tagIDs) > 0 {
		valueStrings := make([]string, 0, len(tagIDs))
		args := make([]interface{}, 0, len(tagIDs)*2)

		for i, tagID := range tagIDs {
			valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, NOW())", i*2+1, i*2+2))
			args = append(args, routeID, tagID)
		}

		insertQuery := fmt.Sprintf(`
			INSERT INTO %s (route_id, tag_id, created_at)
			VALUES %s
		`, r.routeTagsTable(), strings.Join(valueStrings, ","))

		_, err = tx.ExecContext(ctx, insertQuery, args...)
		if err != nil {
			r.log.Error("TagRepository.ReplaceForRoute: ошибка вставки новых связей", "error", err)
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("TagRepository.ReplaceForRoute: ошибка коммита", "error", err)
		return err
	}

	// Обновляем счётчики (вне транзакции, чтобы не блокировать)
	for _, tagID := range oldTagIDs {
		if err := r.DecrementCount(ctx, tagID); err != nil {
			r.log.Error("TagRepository.ReplaceForRoute: ошибка декремента старого тега", "error", err, "tag_id", tagID)
		}
	}

	for _, tagID := range tagIDs {
		if err := r.IncrementCount(ctx, tagID); err != nil {
			r.log.Error("TagRepository.ReplaceForRoute: ошибка инкремента нового тега", "error", err, "tag_id", tagID)
		}
	}

	r.log.Debug("TagRepository.ReplaceForRoute: теги заменены", "route_id", routeID, "new_count", len(tagIDs))
	return nil
}

func (r *TagRepository) IncrementCount(ctx context.Context, tagID uuid.UUID) error {
	query := fmt.Sprintf("UPDATE %s SET routes_count = routes_count + 1 WHERE id = $1", r.table())
	_, err := r.db.ExecContext(ctx, query, tagID)
	if err != nil {
		r.log.Error("TagRepository.IncrementCount: ошибка", "error", err, "tag_id", tagID)
		return err
	}
	return nil
}

func (r *TagRepository) DecrementCount(ctx context.Context, tagID uuid.UUID) error {
	query := fmt.Sprintf("UPDATE %s SET routes_count = GREATEST(0, routes_count - 1) WHERE id = $1", r.table())
	_, err := r.db.ExecContext(ctx, query, tagID)
	if err != nil {
		r.log.Error("TagRepository.DecrementCount: ошибка", "error", err, "tag_id", tagID)
		return err
	}
	return nil
}
