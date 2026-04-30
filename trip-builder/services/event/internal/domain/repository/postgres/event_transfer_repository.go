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

type EventTransferRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewEventTransferRepository(db *sql.DB, schema string, log *logger.Logger) *EventTransferRepository {
	return &EventTransferRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *EventTransferRepository) table() string {
	return fmt.Sprintf("%s.event_transfers", r.schema)
}

func (r *EventTransferRepository) Create(ctx context.Context, transfer *models.EventTransfer) error {
	r.log.Debug("EventTransferRepository.Create: начало",
		"event_id", transfer.EventID,
		"from", transfer.FromWaypointID,
		"to", transfer.ToWaypointID,
	)

	query := fmt.Sprintf(`
		INSERT INTO %s (id, event_id, from_waypoint_id, to_waypoint_id, 
			duration_hours, duration_minutes, transport_type, 
			planned_start, planned_end, actual_start, actual_end, order_index, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		transfer.ID,
		transfer.EventID,
		transfer.FromWaypointID,
		transfer.ToWaypointID,
		transfer.DurationHours,
		transfer.DurationMinutes,
		transfer.TransportType,
		transfer.PlannedStart,
		transfer.PlannedEnd,
		transfer.ActualStart,
		transfer.ActualEnd,
		transfer.OrderIndex,
		transfer.CreatedAt,
	)
	if err != nil {
		r.log.Error("EventTransferRepository.Create: ошибка создания", "error", err)
		return err
	}

	r.log.Debug("EventTransferRepository.Create: перемещение создано",
		"transfer_id", transfer.ID,
	)

	return nil
}

func (r *EventTransferRepository) CreateBatch(ctx context.Context, transfers []models.EventTransfer) error {
	if len(transfers) == 0 {
		r.log.Debug("EventTransferRepository.CreateBatch: пустой список, пропускаем")
		return nil
	}

	r.log.Debug("EventTransferRepository.CreateBatch: начало", "count", len(transfers))

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("EventTransferRepository.CreateBatch: ошибка начала транзакции", "error", err)
		return err
	}
	defer tx.Rollback()

	query := fmt.Sprintf(`
		INSERT INTO %s (id, event_id, from_waypoint_id, to_waypoint_id, 
			duration_hours, duration_minutes, transport_type, 
			planned_start, planned_end, actual_start, actual_end, order_index, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, r.table())

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		r.log.Error("EventTransferRepository.CreateBatch: ошибка подготовки запроса", "error", err)
		return err
	}
	defer stmt.Close()

	for _, transfer := range transfers {
		_, err = stmt.ExecContext(ctx,
			transfer.ID,
			transfer.EventID,
			transfer.FromWaypointID,
			transfer.ToWaypointID,
			transfer.DurationHours,
			transfer.DurationMinutes,
			transfer.TransportType,
			transfer.PlannedStart,
			transfer.PlannedEnd,
			transfer.ActualStart,
			transfer.ActualEnd,
			transfer.OrderIndex,
			transfer.CreatedAt,
		)
		if err != nil {
			r.log.Error("EventTransferRepository.CreateBatch: ошибка вставки",
				"error", err,
				"transfer_id", transfer.ID,
			)
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("EventTransferRepository.CreateBatch: ошибка коммита", "error", err)
		return err
	}

	r.log.Info("EventTransferRepository.CreateBatch: перемещения созданы", "count", len(transfers))

	return nil
}

func (r *EventTransferRepository) GetByEventID(ctx context.Context, eventID uuid.UUID) ([]models.EventTransfer, error) {
	r.log.Debug("EventTransferRepository.GetByEventID: начало", "event_id", eventID)

	query := fmt.Sprintf(`
		SELECT id, event_id, from_waypoint_id, to_waypoint_id, 
			duration_hours, duration_minutes, transport_type, 
			planned_start, planned_end, actual_start, actual_end, order_index, created_at, updated_at
		FROM %s 
		WHERE event_id = $1
		ORDER BY order_index
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, eventID)
	if err != nil {
		r.log.Error("EventTransferRepository.GetByEventID: ошибка запроса", "error", err)
		return nil, err
	}
	defer rows.Close()

	var transfers []models.EventTransfer
	for rows.Next() {
		var transfer models.EventTransfer
		var plannedStart, plannedEnd, actualStart, actualEnd, updatedAt sql.NullTime

		err := rows.Scan(
			&transfer.ID,
			&transfer.EventID,
			&transfer.FromWaypointID,
			&transfer.ToWaypointID,
			&transfer.DurationHours,
			&transfer.DurationMinutes,
			&transfer.TransportType,
			&plannedStart,
			&plannedEnd,
			&actualStart,
			&actualEnd,
			&transfer.OrderIndex,
			&transfer.CreatedAt,
			&updatedAt,
		)
		if err != nil {
			r.log.Error("EventTransferRepository.GetByEventID: ошибка сканирования", "error", err)
			return nil, err
		}

		if plannedStart.Valid {
			transfer.PlannedStart = &plannedStart.Time
		}
		if plannedEnd.Valid {
			transfer.PlannedEnd = &plannedEnd.Time
		}
		if actualStart.Valid {
			transfer.ActualStart = &actualStart.Time
		}
		if actualEnd.Valid {
			transfer.ActualEnd = &actualEnd.Time
		}
		if updatedAt.Valid {
			transfer.UpdatedAt = &updatedAt.Time
		}

		transfers = append(transfers, transfer)
	}

	r.log.Debug("EventTransferRepository.GetByEventID: перемещения получены",
		"event_id", eventID,
		"count", len(transfers),
	)

	return transfers, nil
}

func (r *EventTransferRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.EventTransfer, error) {
	r.log.Debug("EventTransferRepository.GetByID: начало", "id", id)

	query := fmt.Sprintf(`
		SELECT id, event_id, from_waypoint_id, to_waypoint_id, 
			duration_hours, duration_minutes, transport_type, 
			planned_start, planned_end, actual_start, actual_end, order_index, created_at, updated_at
		FROM %s 
		WHERE id = $1
	`, r.table())

	var transfer models.EventTransfer
	var plannedStart, plannedEnd, actualStart, actualEnd, updatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&transfer.ID,
		&transfer.EventID,
		&transfer.FromWaypointID,
		&transfer.ToWaypointID,
		&transfer.DurationHours,
		&transfer.DurationMinutes,
		&transfer.TransportType,
		&plannedStart,
		&plannedEnd,
		&actualStart,
		&actualEnd,
		&transfer.OrderIndex,
		&transfer.CreatedAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Debug("EventTransferRepository.GetByID: перемещение не найдено", "id", id)
			return nil, nil
		}
		r.log.Error("EventTransferRepository.GetByID: ошибка", "error", err)
		return nil, err
	}

	if plannedStart.Valid {
		transfer.PlannedStart = &plannedStart.Time
	}
	if plannedEnd.Valid {
		transfer.PlannedEnd = &plannedEnd.Time
	}
	if actualStart.Valid {
		transfer.ActualStart = &actualStart.Time
	}
	if actualEnd.Valid {
		transfer.ActualEnd = &actualEnd.Time
	}
	if updatedAt.Valid {
		transfer.UpdatedAt = &updatedAt.Time
	}

	return &transfer, nil
}

func (r *EventTransferRepository) GetByWaypoints(ctx context.Context, fromWaypointID, toWaypointID uuid.UUID) (*models.EventTransfer, error) {
	r.log.Debug("EventTransferRepository.GetByWaypoints: начало",
		"from", fromWaypointID,
		"to", toWaypointID,
	)

	query := fmt.Sprintf(`
		SELECT id, event_id, from_waypoint_id, to_waypoint_id, 
			duration_hours, duration_minutes, transport_type, 
			planned_start, planned_end, actual_start, actual_end, order_index, created_at, updated_at
		FROM %s 
		WHERE from_waypoint_id = $1 AND to_waypoint_id = $2
	`, r.table())

	var transfer models.EventTransfer
	var plannedStart, plannedEnd, actualStart, actualEnd, updatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, fromWaypointID, toWaypointID).Scan(
		&transfer.ID,
		&transfer.EventID,
		&transfer.FromWaypointID,
		&transfer.ToWaypointID,
		&transfer.DurationHours,
		&transfer.DurationMinutes,
		&transfer.TransportType,
		&plannedStart,
		&plannedEnd,
		&actualStart,
		&actualEnd,
		&transfer.OrderIndex,
		&transfer.CreatedAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("EventTransferRepository.GetByWaypoints: ошибка", "error", err)
		return nil, err
	}

	if plannedStart.Valid {
		transfer.PlannedStart = &plannedStart.Time
	}
	if plannedEnd.Valid {
		transfer.PlannedEnd = &plannedEnd.Time
	}
	if actualStart.Valid {
		transfer.ActualStart = &actualStart.Time
	}
	if actualEnd.Valid {
		transfer.ActualEnd = &actualEnd.Time
	}
	if updatedAt.Valid {
		transfer.UpdatedAt = &updatedAt.Time
	}

	return &transfer, nil
}

func (r *EventTransferRepository) Update(ctx context.Context, transfer *models.EventTransfer) error {
	r.log.Debug("EventTransferRepository.Update: начало", "id", transfer.ID)

	query := fmt.Sprintf(`
		UPDATE %s SET
			duration_hours = $1,
			duration_minutes = $2,
			transport_type = $3,
			planned_start = $4,
			planned_end = $5,
			actual_start = $6,
			actual_end = $7,
			updated_at = NOW()
		WHERE id = $8
	`, r.table())

	result, err := r.db.ExecContext(ctx, query,
		transfer.DurationHours,
		transfer.DurationMinutes,
		transfer.TransportType,
		transfer.PlannedStart,
		transfer.PlannedEnd,
		transfer.ActualStart,
		transfer.ActualEnd,
		transfer.ID,
	)
	if err != nil {
		r.log.Error("EventTransferRepository.Update: ошибка обновления", "error", err)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("EventTransferRepository.Update: перемещение обновлено",
		"id", transfer.ID,
		"rows_affected", rows,
	)

	return nil
}

func (r *EventTransferRepository) UpdateActualTimes(ctx context.Context, id uuid.UUID, actualStart, actualEnd *time.Time) error {
	r.log.Debug("EventTransferRepository.UpdateActualTimes: начало", "id", id)

	query := fmt.Sprintf(`
		UPDATE %s SET
			actual_start = COALESCE($1, actual_start),
			actual_end = COALESCE($2, actual_end),
			updated_at = NOW()
		WHERE id = $3
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, actualStart, actualEnd, id)
	if err != nil {
		r.log.Error("EventTransferRepository.UpdateActualTimes: ошибка", "error", err)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("EventTransferRepository.UpdateActualTimes: обновлено",
		"id", id,
		"rows_affected", rows,
	)

	return nil
}

func (r *EventTransferRepository) DeleteByEventID(ctx context.Context, eventID uuid.UUID) error {
	r.log.Debug("EventTransferRepository.DeleteByEventID: начало", "event_id", eventID)

	query := fmt.Sprintf("DELETE FROM %s WHERE event_id = $1", r.table())

	result, err := r.db.ExecContext(ctx, query, eventID)
	if err != nil {
		r.log.Error("EventTransferRepository.DeleteByEventID: ошибка удаления", "error", err)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("EventTransferRepository.DeleteByEventID: перемещения удалены",
		"event_id", eventID,
		"rows_affected", rows,
	)

	return nil
}

func (r *EventTransferRepository) Delete(ctx context.Context, id uuid.UUID) error {
	r.log.Debug("EventTransferRepository.Delete: начало", "id", id)

	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.table())

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error("EventTransferRepository.Delete: ошибка удаления", "error", err)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("EventTransferRepository.Delete: перемещение удалено",
		"id", id,
		"rows_affected", rows,
	)

	return nil
}