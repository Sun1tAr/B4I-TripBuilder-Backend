package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type RouteRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewRouteRepository(db *sql.DB, schema string, log *logger.Logger) *RouteRepository {
	return &RouteRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *RouteRepository) table() string {
	return fmt.Sprintf("%s.routes", r.schema)
}

func (r *RouteRepository) waypointsTable() string {
	return fmt.Sprintf("%s.waypoints", r.schema)
}

func (r *RouteRepository) Create(ctx context.Context, route *models.Route, waypoints []models.Waypoint) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("RouteRepository.Create: ошибка начала транзакции", "error", err)
		return err
	}
	defer tx.Rollback()

	// Вставляем маршрут
	routeQuery := fmt.Sprintf(`
		INSERT INTO %s (id, title, description, country, image_url, user_id, is_public, 
		                total_days, total_hours, likes_count, events_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, r.table())

	_, err = tx.ExecContext(ctx, routeQuery,
		route.ID, route.Title, route.Description, route.Country, route.ImageURL,
		route.UserID, route.IsPublic, route.TotalDays, route.TotalHours,
		route.LikesCount, route.EventsCount, route.CreatedAt,
	)
	if err != nil {
		r.log.Error("RouteRepository.Create: ошибка вставки маршрута", "error", err)
		return err
	}

	// Вставляем точки маршрута
	if len(waypoints) > 0 {
		waypointQuery := fmt.Sprintf(`
			INSERT INTO %s (id, route_id, city, description, latitude, longitude, 
			                duration_days, duration_hours, order_index, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, r.waypointsTable())

		for _, wp := range waypoints {
			_, err = tx.ExecContext(ctx, waypointQuery,
				wp.ID, wp.RouteID, wp.City, wp.Description, wp.Latitude, wp.Longitude,
				wp.DurationDays, wp.DurationHours, wp.OrderIndex, wp.CreatedAt,
			)
			if err != nil {
				r.log.Error("RouteRepository.Create: ошибка вставки точки", "error", err, "waypoint_id", wp.ID)
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("RouteRepository.Create: ошибка коммита", "error", err)
		return err
	}

	r.log.Debug("RouteRepository.Create: маршрут создан", "route_id", route.ID)
	return nil
}

func (r *RouteRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Route, error) {
	query := fmt.Sprintf(`
		SELECT id, title, description, country, image_url, user_id, is_public,
		       total_days, total_hours, likes_count, events_count, created_at, updated_at
		FROM %s WHERE id = $1
	`, r.table())

	route := &models.Route{}
	var updatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&route.ID, &route.Title, &route.Description, &route.Country, &route.ImageURL,
		&route.UserID, &route.IsPublic, &route.TotalDays, &route.TotalHours,
		&route.LikesCount, &route.EventsCount, &route.CreatedAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("RouteRepository.GetByID: ошибка", "error", err, "route_id", id)
		return nil, err
	}

	if updatedAt.Valid {
		route.UpdatedAt = &updatedAt.Time
	}

	return route, nil
}

func (r *RouteRepository) GetByIDWithDetails(ctx context.Context, id uuid.UUID) (*models.Route, []models.Waypoint, error) {
	// Получаем маршрут
	route, err := r.GetByID(ctx, id)
	if err != nil || route == nil {
		return route, nil, err
	}

	// Получаем точки
	waypointsQuery := fmt.Sprintf(`
		SELECT id, route_id, city, description, latitude, longitude,
		       duration_days, duration_hours, order_index, created_at, updated_at
		FROM %s WHERE route_id = $1 ORDER BY order_index
	`, r.waypointsTable())

	rows, err := r.db.QueryContext(ctx, waypointsQuery, id)
	if err != nil {
		r.log.Error("RouteRepository.GetByIDWithDetails: ошибка получения точек", "error", err, "route_id", id)
		return route, nil, err
	}
	defer rows.Close()

	var waypoints []models.Waypoint
	for rows.Next() {
		var wp models.Waypoint
		var updatedAt sql.NullTime

		err := rows.Scan(
			&wp.ID, &wp.RouteID, &wp.City, &wp.Description, &wp.Latitude, &wp.Longitude,
			&wp.DurationDays, &wp.DurationHours, &wp.OrderIndex, &wp.CreatedAt, &updatedAt,
		)
		if err != nil {
			r.log.Error("RouteRepository.GetByIDWithDetails: ошибка сканирования", "error", err)
			return route, nil, err
		}

		if updatedAt.Valid {
			wp.UpdatedAt = &updatedAt.Time
		}
		waypoints = append(waypoints, wp)
	}

	return route, waypoints, nil
}

func (r *RouteRepository) ListPublic(ctx context.Context, limit, offset int, sort string, filters map[string]interface{}) ([]models.Route, int64, error) {
	// Базовый запрос
	baseQuery := fmt.Sprintf("FROM %s WHERE is_public = true", r.table())
	args := []interface{}{}
	argPos := 1

	// Применяем фильтры
	if country, ok := filters["country"]; ok && country != "" {
		baseQuery += fmt.Sprintf(" AND country = $%d", argPos)
		args = append(args, country)
		argPos++
	}

	if durationFrom, ok := filters["duration_from"]; ok {
		totalDays := durationFrom.(int)
		baseQuery += fmt.Sprintf(" AND (total_days * 24 + total_hours) >= $%d", argPos)
		args = append(args, totalDays*24)
		argPos++
	}

	if durationTo, ok := filters["duration_to"]; ok {
		totalDays := durationTo.(int)
		baseQuery += fmt.Sprintf(" AND (total_days * 24 + total_hours) <= $%d", argPos)
		args = append(args, totalDays*24)
		argPos++
	}

	// Сортировка
	orderBy := " ORDER BY created_at DESC"
	switch sort {
	case "likes":
		orderBy = " ORDER BY likes_count DESC, created_at DESC"
	case "popular":
		orderBy = " ORDER BY (likes_count + events_count) DESC, created_at DESC"
	}

	// Получаем общее количество
	countQuery := "SELECT COUNT(*) " + baseQuery
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		r.log.Error("RouteRepository.ListPublic: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	// Получаем данные
	query := "SELECT id, title, description, country, image_url, user_id, is_public, " +
		"total_days, total_hours, likes_count, events_count, created_at, updated_at " +
		baseQuery + orderBy + fmt.Sprintf(" LIMIT $%d OFFSET $%d", argPos, argPos+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		r.log.Error("RouteRepository.ListPublic: ошибка запроса", "error", err)
		return nil, 0, err
	}
	defer rows.Close()

	var routes []models.Route
	for rows.Next() {
		var route models.Route
		var updatedAt sql.NullTime

		err := rows.Scan(
			&route.ID, &route.Title, &route.Description, &route.Country, &route.ImageURL,
			&route.UserID, &route.IsPublic, &route.TotalDays, &route.TotalHours,
			&route.LikesCount, &route.EventsCount, &route.CreatedAt, &updatedAt,
		)
		if err != nil {
			r.log.Error("RouteRepository.ListPublic: ошибка сканирования", "error", err)
			return nil, 0, err
		}

		if updatedAt.Valid {
			route.UpdatedAt = &updatedAt.Time
		}
		routes = append(routes, route)
	}

	return routes, total, nil
}

func (r *RouteRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Route, int64, error) {
	query := fmt.Sprintf(`
		SELECT id, title, description, country, image_url, user_id, is_public,
		       total_days, total_hours, likes_count, events_count, created_at, updated_at
		FROM %s WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		r.log.Error("RouteRepository.ListByUserID: ошибка запроса", "error", err, "user_id", userID)
		return nil, 0, err
	}
	defer rows.Close()

	var routes []models.Route
	for rows.Next() {
		var route models.Route
		var updatedAt sql.NullTime

		err := rows.Scan(
			&route.ID, &route.Title, &route.Description, &route.Country, &route.ImageURL,
			&route.UserID, &route.IsPublic, &route.TotalDays, &route.TotalHours,
			&route.LikesCount, &route.EventsCount, &route.CreatedAt, &updatedAt,
		)
		if err != nil {
			r.log.Error("RouteRepository.ListByUserID: ошибка сканирования", "error", err)
			return nil, 0, err
		}

		if updatedAt.Valid {
			route.UpdatedAt = &updatedAt.Time
		}
		routes = append(routes, route)
	}

	// Получаем общее количество
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE user_id = $1", r.table())
	var total int64
	err = r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)
	if err != nil {
		r.log.Error("RouteRepository.ListByUserID: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	return routes, total, nil
}

func (r *RouteRepository) ListByTag(ctx context.Context, tagName string, limit, offset int) ([]models.Route, int64, error) {
	tagsTable := fmt.Sprintf("%s.tags", r.schema)
	routeTagsTable := fmt.Sprintf("%s.route_tags", r.schema)

	// Получаем маршруты по тегу
	query := fmt.Sprintf(`
		SELECT r.id, r.title, r.description, r.country, r.image_url, r.user_id, r.is_public,
		       r.total_days, r.total_hours, r.likes_count, r.events_count, r.created_at, r.updated_at
		FROM %s r
		JOIN %s rt ON r.id = rt.route_id
		JOIN %s t ON rt.tag_id = t.id
		WHERE t.name = $1
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3
	`, r.table(), routeTagsTable, tagsTable)

	rows, err := r.db.QueryContext(ctx, query, tagName, limit, offset)
	if err != nil {
		r.log.Error("RouteRepository.ListByTag: ошибка запроса", "error", err, "tag", tagName)
		return nil, 0, err
	}
	defer rows.Close()

	var routes []models.Route
	for rows.Next() {
		var route models.Route
		var updatedAt sql.NullTime

		err := rows.Scan(
			&route.ID, &route.Title, &route.Description, &route.Country, &route.ImageURL,
			&route.UserID, &route.IsPublic, &route.TotalDays, &route.TotalHours,
			&route.LikesCount, &route.EventsCount, &route.CreatedAt, &updatedAt,
		)
		if err != nil {
			r.log.Error("RouteRepository.ListByTag: ошибка сканирования", "error", err)
			return nil, 0, err
		}

		if updatedAt.Valid {
			route.UpdatedAt = &updatedAt.Time
		}
		routes = append(routes, route)
	}

	// Получаем общее количество
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s r
		JOIN %s rt ON r.id = rt.route_id
		JOIN %s t ON rt.tag_id = t.id
		WHERE t.name = $1
	`, r.table(), routeTagsTable, tagsTable)

	var total int64
	err = r.db.QueryRowContext(ctx, countQuery, tagName).Scan(&total)
	if err != nil {
		r.log.Error("RouteRepository.ListByTag: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	return routes, total, nil
}

func (r *RouteRepository) GetPopular(ctx context.Context, limit int) ([]models.Route, error) {
	query := fmt.Sprintf(`
		SELECT id, title, description, country, image_url, user_id, is_public,
		       total_days, total_hours, likes_count, events_count, created_at, updated_at
		FROM %s
		WHERE is_public = true
		ORDER BY (likes_count + events_count) DESC, created_at DESC
		LIMIT $1
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		r.log.Error("RouteRepository.GetPopular: ошибка запроса", "error", err)
		return nil, err
	}
	defer rows.Close()

	var routes []models.Route
	for rows.Next() {
		var route models.Route
		var updatedAt sql.NullTime

		err := rows.Scan(
			&route.ID, &route.Title, &route.Description, &route.Country, &route.ImageURL,
			&route.UserID, &route.IsPublic, &route.TotalDays, &route.TotalHours,
			&route.LikesCount, &route.EventsCount, &route.CreatedAt, &updatedAt,
		)
		if err != nil {
			r.log.Error("RouteRepository.GetPopular: ошибка сканирования", "error", err)
			return nil, err
		}

		if updatedAt.Valid {
			route.UpdatedAt = &updatedAt.Time
		}
		routes = append(routes, route)
	}

	return routes, nil
}

func (r *RouteRepository) Update(ctx context.Context, route *models.Route, waypoints []models.Waypoint) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("RouteRepository.Update: ошибка начала транзакции", "error", err)
		return err
	}
	defer tx.Rollback()

	// Обновляем маршрут
	routeQuery := fmt.Sprintf(`
		UPDATE %s SET
			title = $1,
			description = $2,
			country = $3,
			image_url = $4,
			is_public = $5,
			total_days = $6,
			total_hours = $7,
			updated_at = NOW()
		WHERE id = $8
	`, r.table())

	_, err = tx.ExecContext(ctx, routeQuery,
		route.Title, route.Description, route.Country, route.ImageURL,
		route.IsPublic, route.TotalDays, route.TotalHours, route.ID,
	)
	if err != nil {
		r.log.Error("RouteRepository.Update: ошибка обновления маршрута", "error", err, "route_id", route.ID)
		return err
	}

	// Удаляем старые точки
	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE route_id = $1", r.waypointsTable())
	_, err = tx.ExecContext(ctx, deleteQuery, route.ID)
	if err != nil {
		r.log.Error("RouteRepository.Update: ошибка удаления старых точек", "error", err, "route_id", route.ID)
		return err
	}

	// Вставляем новые точки
	if len(waypoints) > 0 {
		waypointQuery := fmt.Sprintf(`
			INSERT INTO %s (id, route_id, city, description, latitude, longitude, 
			                duration_days, duration_hours, order_index, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, r.waypointsTable())

		for _, wp := range waypoints {
			_, err = tx.ExecContext(ctx, waypointQuery,
				wp.ID, wp.RouteID, wp.City, wp.Description, wp.Latitude, wp.Longitude,
				wp.DurationDays, wp.DurationHours, wp.OrderIndex, wp.CreatedAt, wp.UpdatedAt,
			)
			if err != nil {
				r.log.Error("RouteRepository.Update: ошибка вставки точки", "error", err, "waypoint_id", wp.ID)
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("RouteRepository.Update: ошибка коммита", "error", err)
		return err
	}

	r.log.Debug("RouteRepository.Update: маршрут обновлён", "route_id", route.ID)
	return nil
}

func (r *RouteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.table())
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error("RouteRepository.Delete: ошибка удаления", "error", err, "route_id", id)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("RouteRepository.Delete: маршрут удалён", "route_id", id, "rows_affected", rows)
	return nil
}

func (r *RouteRepository) IsOwner(ctx context.Context, routeID, userID uuid.UUID) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1 AND user_id = $2)", r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, routeID, userID).Scan(&exists)
	if err != nil {
		r.log.Error("RouteRepository.IsOwner: ошибка проверки", "error", err, "route_id", routeID, "user_id", userID)
		return false, err
	}
	return exists, nil
}

func (r *RouteRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1)", r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		r.log.Error("RouteRepository.Exists: ошибка проверки", "error", err, "route_id", id)
		return false, err
	}
	return exists, nil
}

func (r *RouteRepository) IncrementEventsCount(ctx context.Context, routeID uuid.UUID) error {
	query := fmt.Sprintf("UPDATE %s SET events_count = events_count + 1 WHERE id = $1", r.table())
	_, err := r.db.ExecContext(ctx, query, routeID)
	if err != nil {
		r.log.Error("RouteRepository.IncrementEventsCount: ошибка", "error", err, "route_id", routeID)
		return err
	}
	return nil
}
