package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type EventRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewEventRepository(db *sql.DB, schema string, log *logger.Logger) *EventRepository {
	return &EventRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *EventRepository) table() string {
	return fmt.Sprintf("%s.events", r.schema)
}

func (r *EventRepository) Create(ctx context.Context, event *models.Event) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (id, route_id, title, description, owner_id, start_date, end_date, 
		                is_group, status, is_public, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		event.ID, event.RouteID, event.Title, event.Description, event.OwnerID,
		event.StartDate, event.EndDate, event.IsGroup, event.Status, event.IsPublic, event.CreatedAt,
	)
	if err != nil {
		r.log.Error("EventRepository.Create: ошибка создания", "error", err, "event_id", event.ID)
		return err
	}

	r.log.Debug("EventRepository.Create: событие создано", "event_id", event.ID)
	return nil
}

func (r *EventRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Event, error) {
	query := fmt.Sprintf(`
		SELECT id, route_id, title, description, owner_id, start_date, end_date,
		       is_group, status, is_public, created_at, updated_at
		FROM %s WHERE id = $1
	`, r.table())

	event := &models.Event{}
	var updatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID, &event.RouteID, &event.Title, &event.Description, &event.OwnerID,
		&event.StartDate, &event.EndDate, &event.IsGroup, &event.Status, &event.IsPublic,
		&event.CreatedAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("EventRepository.GetByID: ошибка", "error", err, "event_id", id)
		return nil, err
	}

	if updatedAt.Valid {
		event.UpdatedAt = &updatedAt.Time
	}

	return event, nil
}

func (r *EventRepository) GetByIDWithDetails(ctx context.Context, id uuid.UUID) (*models.Event, error) {
	return r.GetByID(ctx, id)
}

// ListByUserID возвращает поездки, где пользователь является полноценным участником (оба флага true)
func (r *EventRepository) ListByUserID(ctx context.Context, userID uuid.UUID, status string, limit, offset int) ([]models.Event, int64, error) {
	participantsTable := fmt.Sprintf("%s.event_participants", r.schema)

	args := []interface{}{userID}
	argPos := 2
	statusFilter := ""

	if status != "" {
		statusFilter = fmt.Sprintf(" AND e.status = $%d", argPos)
		args = append(args, status)
		argPos++
	}

	// Условие: участник полностью подтверждён (approved_by_owner = true AND approved_by_user = true)
	query := fmt.Sprintf(`
		SELECT e.id, e.route_id, e.title, e.description, e.owner_id, e.start_date, e.end_date,
		       e.is_group, e.status, e.is_public, e.created_at, e.updated_at
		FROM %s e
		JOIN %s p ON e.id = p.event_id
		WHERE p.user_id = $1 
		  AND p.approved_by_owner = true 
		  AND p.approved_by_user = true%s
		ORDER BY e.start_date DESC
		LIMIT $%d OFFSET $%d
	`, r.table(), participantsTable, statusFilter, argPos, argPos+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		r.log.Error("EventRepository.ListByUserID: ошибка запроса", "error", err, "user_id", userID)
		return nil, 0, err
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var event models.Event
		var updatedAt sql.NullTime

		err := rows.Scan(
			&event.ID, &event.RouteID, &event.Title, &event.Description, &event.OwnerID,
			&event.StartDate, &event.EndDate, &event.IsGroup, &event.Status, &event.IsPublic,
			&event.CreatedAt, &updatedAt,
		)
		if err != nil {
			r.log.Error("EventRepository.ListByUserID: ошибка сканирования", "error", err)
			return nil, 0, err
		}

		if updatedAt.Valid {
			event.UpdatedAt = &updatedAt.Time
		}
		events = append(events, event)
	}

	// Получаем общее количество
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s e
		JOIN %s p ON e.id = p.event_id
		WHERE p.user_id = $1 
		  AND p.approved_by_owner = true 
		  AND p.approved_by_user = true%s
	`, r.table(), participantsTable, statusFilter)

	countArgs := []interface{}{userID}
	if status != "" {
		countArgs = append(countArgs, status)
	}

	var total int64
	err = r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		r.log.Error("EventRepository.ListByUserID: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	return events, total, nil
}

func (r *EventRepository) ListPublic(ctx context.Context, limit, offset int) ([]models.Event, int64, error) {
	query := fmt.Sprintf(`
		SELECT id, route_id, title, description, owner_id, start_date, end_date,
		       is_group, status, is_public, created_at, updated_at
		FROM %s
		WHERE is_public = true AND status = 'completed'
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		r.log.Error("EventRepository.ListPublic: ошибка запроса", "error", err)
		return nil, 0, err
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var event models.Event
		var updatedAt sql.NullTime

		err := rows.Scan(
			&event.ID, &event.RouteID, &event.Title, &event.Description, &event.OwnerID,
			&event.StartDate, &event.EndDate, &event.IsGroup, &event.Status, &event.IsPublic,
			&event.CreatedAt, &updatedAt,
		)
		if err != nil {
			r.log.Error("EventRepository.ListPublic: ошибка сканирования", "error", err)
			return nil, 0, err
		}

		if updatedAt.Valid {
			event.UpdatedAt = &updatedAt.Time
		}
		events = append(events, event)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE is_public = true AND status = 'completed'", r.table())
	var total int64
	err = r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		r.log.Error("EventRepository.ListPublic: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	return events, total, nil
}

func (r *EventRepository) Update(ctx context.Context, event *models.Event) error {
	query := fmt.Sprintf(`
		UPDATE %s SET
			title = $1,
			description = $2,
			start_date = $3,
			end_date = $4,
			is_public = $5,
			updated_at = NOW()
		WHERE id = $6
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		event.Title, event.Description, event.StartDate, event.EndDate,
		event.IsPublic, event.ID,
	)
	if err != nil {
		r.log.Error("EventRepository.Update: ошибка обновления", "error", err, "event_id", event.ID)
		return err
	}

	r.log.Debug("EventRepository.Update: событие обновлено", "event_id", event.ID)
	return nil
}

func (r *EventRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := fmt.Sprintf("UPDATE %s SET status = $1, updated_at = NOW() WHERE id = $2", r.table())
	_, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		r.log.Error("EventRepository.UpdateStatus: ошибка", "error", err, "event_id", id, "status", status)
		return err
	}
	return nil
}

func (r *EventRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.table())
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error("EventRepository.Delete: ошибка удаления", "error", err, "event_id", id)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("EventRepository.Delete: событие удалено", "event_id", id, "rows", rows)
	return nil
}

func (r *EventRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1)", r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		r.log.Error("EventRepository.Exists: ошибка", "error", err, "event_id", id)
		return false, err
	}
	return exists, nil
}

func (r *EventRepository) IsOwner(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1 AND owner_id = $2)", r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, eventID, userID).Scan(&exists)
	if err != nil {
		r.log.Error("EventRepository.IsOwner: ошибка", "error", err, "event_id", eventID, "user_id", userID)
		return false, err
	}
	return exists, nil
}

func (r *EventRepository) GetActiveEvents(ctx context.Context, beforeDate time.Time) ([]uuid.UUID, error) {
	query := fmt.Sprintf(`
		SELECT id FROM %s 
		WHERE status = 'active' AND end_date < $1
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, beforeDate)
	if err != nil {
		r.log.Error("EventRepository.GetActiveEvents: ошибка", "error", err)
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			r.log.Error("EventRepository.GetActiveEvents: ошибка сканирования", "error", err)
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// GetPlannedEventsForActivation возвращает planned события, которые должны стать active
func (r *EventRepository) GetPlannedEventsForActivation(ctx context.Context, beforeDate time.Time) ([]uuid.UUID, error) {
	query := fmt.Sprintf(`
		SELECT id FROM %s 
		WHERE status = 'planned' AND start_date <= $1
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, beforeDate)
	if err != nil {
		r.log.Error("EventRepository.GetPlannedEventsForActivation: ошибка", "error", err)
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			r.log.Error("EventRepository.GetPlannedEventsForActivation: ошибка сканирования", "error", err)
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// CreateWithWaypoints создаёт событие и его точки в одной транзакции
func (r *EventRepository) CreateWithWaypoints(ctx context.Context, event *models.Event, waypoints []models.Waypoint) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("EventRepository.CreateWithWaypoints: ошибка начала транзакции", "error", err)
		return err
	}
	defer tx.Rollback()

	// Вставляем событие
	eventQuery := fmt.Sprintf(`
        INSERT INTO %s (id, route_id, title, description, owner_id, start_date, end_date, 
                        is_group, status, is_public, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    `, r.table())

	_, err = tx.ExecContext(ctx, eventQuery,
		event.ID, event.RouteID, event.Title, event.Description, event.OwnerID,
		event.StartDate, event.EndDate, event.IsGroup, event.Status, event.IsPublic, event.CreatedAt,
	)
	if err != nil {
		r.log.Error("EventRepository.CreateWithWaypoints: ошибка вставки события", "error", err, "event_id", event.ID)
		return err
	}

	// Вставляем точки, если есть
	if len(waypoints) > 0 {
		waypointTable := fmt.Sprintf("%s.event_waypoints", r.schema)
		waypointQuery := fmt.Sprintf(`
            INSERT INTO %s (id, event_id, route_waypoint_id, city, description, latitude, longitude,
                            planned_start, planned_end, status, order_index, created_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
        `, waypointTable)

		for _, wp := range waypoints {
			_, err = tx.ExecContext(ctx, waypointQuery,
				wp.ID, wp.EventID, wp.RouteWaypointID, wp.City, wp.Description,
				wp.Latitude, wp.Longitude, wp.PlannedStart, wp.PlannedEnd,
				wp.Status, wp.OrderIndex, wp.CreatedAt,
			)
			if err != nil {
				r.log.Error("EventRepository.CreateWithWaypoints: ошибка вставки точки", "error", err, "waypoint_id", wp.ID)
				return err
			}
		}
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		r.log.Error("EventRepository.CreateWithWaypoints: ошибка коммита", "error", err)
		return err
	}

	r.log.Debug("EventRepository.CreateWithWaypoints: событие и точки созданы", "event_id", event.ID, "waypoints_count", len(waypoints))
	return nil
}
