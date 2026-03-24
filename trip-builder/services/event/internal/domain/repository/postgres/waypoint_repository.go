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

type WaypointRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewWaypointRepository(db *sql.DB, schema string, log *logger.Logger) *WaypointRepository {
	return &WaypointRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *WaypointRepository) table() string {
	return fmt.Sprintf("%s.event_waypoints", r.schema)
}

func (r *WaypointRepository) Create(ctx context.Context, waypoint *models.Waypoint) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (id, event_id, route_waypoint_id, city, description, latitude, longitude,
		                planned_start, planned_end, status, order_index, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		waypoint.ID, waypoint.EventID, waypoint.RouteWaypointID, waypoint.City,
		waypoint.Description, waypoint.Latitude, waypoint.Longitude,
		waypoint.PlannedStart, waypoint.PlannedEnd, waypoint.Status,
		waypoint.OrderIndex, waypoint.CreatedAt,
	)
	if err != nil {
		r.log.Error("WaypointRepository.Create: ошибка создания", "error", err, "waypoint_id", waypoint.ID)
		return err
	}

	return nil
}

func (r *WaypointRepository) GetByEventID(ctx context.Context, eventID uuid.UUID) ([]models.Waypoint, error) {
	query := fmt.Sprintf(`
		SELECT id, event_id, route_waypoint_id, city, description, latitude, longitude,
		       planned_start, planned_end, actual_start, actual_end, status, order_index,
		       created_at, updated_at
		FROM %s WHERE event_id = $1 ORDER BY order_index
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, eventID)
	if err != nil {
		r.log.Error("WaypointRepository.GetByEventID: ошибка запроса", "error", err, "event_id", eventID)
		return nil, err
	}
	defer rows.Close()

	var waypoints []models.Waypoint
	for rows.Next() {
		var wp models.Waypoint
		var routeWaypointID sql.NullString
		var plannedStart, plannedEnd, actualStart, actualEnd, updatedAt sql.NullTime

		err := rows.Scan(
			&wp.ID, &wp.EventID, &routeWaypointID, &wp.City, &wp.Description,
			&wp.Latitude, &wp.Longitude, &plannedStart, &plannedEnd,
			&actualStart, &actualEnd, &wp.Status, &wp.OrderIndex,
			&wp.CreatedAt, &updatedAt,
		)
		if err != nil {
			r.log.Error("WaypointRepository.GetByEventID: ошибка сканирования", "error", err)
			return nil, err
		}

		// Преобразуем route_waypoint_id из string в uuid.UUID
		if routeWaypointID.Valid && routeWaypointID.String != "" {
			id, err := uuid.Parse(routeWaypointID.String)
			if err == nil {
				wp.RouteWaypointID = &id
			}
		}

		if plannedStart.Valid {
			wp.PlannedStart = &plannedStart.Time
		}
		if plannedEnd.Valid {
			wp.PlannedEnd = &plannedEnd.Time
		}
		if actualStart.Valid {
			wp.ActualStart = &actualStart.Time
		}
		if actualEnd.Valid {
			wp.ActualEnd = &actualEnd.Time
		}
		if updatedAt.Valid {
			wp.UpdatedAt = &updatedAt.Time
		}

		waypoints = append(waypoints, wp)
	}

	return waypoints, nil
}

func (r *WaypointRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Waypoint, error) {
	query := fmt.Sprintf(`
		SELECT id, event_id, route_waypoint_id, city, description, latitude, longitude,
		       planned_start, planned_end, actual_start, actual_end, status, order_index,
		       created_at, updated_at
		FROM %s WHERE id = $1
	`, r.table())

	wp := &models.Waypoint{}
	var routeWaypointID sql.NullString
	var plannedStart, plannedEnd, actualStart, actualEnd, updatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&wp.ID, &wp.EventID, &routeWaypointID, &wp.City, &wp.Description,
		&wp.Latitude, &wp.Longitude, &plannedStart, &plannedEnd,
		&actualStart, &actualEnd, &wp.Status, &wp.OrderIndex,
		&wp.CreatedAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("WaypointRepository.GetByID: ошибка", "error", err, "waypoint_id", id)
		return nil, err
	}

	// Преобразуем route_waypoint_id из string в uuid.UUID
	if routeWaypointID.Valid && routeWaypointID.String != "" {
		uuidID, err := uuid.Parse(routeWaypointID.String)
		if err == nil {
			wp.RouteWaypointID = &uuidID
		}
	}

	if plannedStart.Valid {
		wp.PlannedStart = &plannedStart.Time
	}
	if plannedEnd.Valid {
		wp.PlannedEnd = &plannedEnd.Time
	}
	if actualStart.Valid {
		wp.ActualStart = &actualStart.Time
	}
	if actualEnd.Valid {
		wp.ActualEnd = &actualEnd.Time
	}
	if updatedAt.Valid {
		wp.UpdatedAt = &updatedAt.Time
	}

	return wp, nil
}

func (r *WaypointRepository) Update(ctx context.Context, waypoint *models.Waypoint) error {
	query := fmt.Sprintf(`
		UPDATE %s SET
			city = $1,
			description = $2,
			latitude = $3,
			longitude = $4,
			planned_start = $5,
			planned_end = $6,
			updated_at = NOW()
		WHERE id = $7
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		waypoint.City, waypoint.Description, waypoint.Latitude, waypoint.Longitude,
		waypoint.PlannedStart, waypoint.PlannedEnd, waypoint.ID,
	)
	if err != nil {
		r.log.Error("WaypointRepository.Update: ошибка обновления", "error", err, "waypoint_id", waypoint.ID)
		return err
	}

	return nil
}

func (r *WaypointRepository) UpdateVisit(ctx context.Context, id uuid.UUID, actualStart, actualEnd *time.Time) error {
	query := fmt.Sprintf(`
		UPDATE %s SET
			actual_start = COALESCE($1, actual_start),
			actual_end = COALESCE($2, actual_end),
			status = 'visited',
			updated_at = NOW()
		WHERE id = $3
	`, r.table())

	_, err := r.db.ExecContext(ctx, query, actualStart, actualEnd, id)
	if err != nil {
		r.log.Error("WaypointRepository.UpdateVisit: ошибка", "error", err, "waypoint_id", id)
		return err
	}

	return nil
}

func (r *WaypointRepository) DeleteByEventID(ctx context.Context, eventID uuid.UUID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE event_id = $1", r.table())
	result, err := r.db.ExecContext(ctx, query, eventID)
	if err != nil {
		r.log.Error("WaypointRepository.DeleteByEventID: ошибка удаления", "error", err, "event_id", eventID)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("WaypointRepository.DeleteByEventID: точки удалены", "event_id", eventID, "rows", rows)
	return nil
}

func (r *WaypointRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1)", r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		r.log.Error("WaypointRepository.Exists: ошибка", "error", err, "waypoint_id", id)
		return false, err
	}
	return exists, nil
}
