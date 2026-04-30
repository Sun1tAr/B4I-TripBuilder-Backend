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

type TransferRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewTransferRepository(db *sql.DB, schema string, log *logger.Logger) *TransferRepository {
	return &TransferRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *TransferRepository) table() string {
	return fmt.Sprintf("%s.route_transfers", r.schema)
}

func (r *TransferRepository) Create(ctx context.Context, transfer *models.Transfer) error {
	r.log.Debug("TransferRepository.Create: начало",
		"route_id", transfer.RouteID,
		"from", transfer.FromWaypointID,
		"to", transfer.ToWaypointID,
	)

	query := fmt.Sprintf(`
		INSERT INTO %s (id, route_id, from_waypoint_id, to_waypoint_id, 
			duration_hours, duration_minutes, transport_type, order_index, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		transfer.ID,
		transfer.RouteID,
		transfer.FromWaypointID,
		transfer.ToWaypointID,
		transfer.DurationHours,
		transfer.DurationMinutes,
		transfer.TransportType,
		transfer.OrderIndex,
		transfer.CreatedAt,
	)
	if err != nil {
		r.log.Error("TransferRepository.Create: ошибка создания", "error", err)
		return err
	}

	r.log.Debug("TransferRepository.Create: перемещение создано",
		"transfer_id", transfer.ID,
	)

	return nil
}

func (r *TransferRepository) CreateBatch(ctx context.Context, transfers []models.Transfer) error {
	if len(transfers) == 0 {
		r.log.Debug("TransferRepository.CreateBatch: пустой список, пропускаем")
		return nil
	}

	r.log.Debug("TransferRepository.CreateBatch: начало", "count", len(transfers))

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("TransferRepository.CreateBatch: ошибка начала транзакции", "error", err)
		return err
	}
	defer tx.Rollback()

	query := fmt.Sprintf(`
		INSERT INTO %s (id, route_id, from_waypoint_id, to_waypoint_id, 
			duration_hours, duration_minutes, transport_type, order_index, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, r.table())

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		r.log.Error("TransferRepository.CreateBatch: ошибка подготовки запроса", "error", err)
		return err
	}
	defer stmt.Close()

	for _, transfer := range transfers {
		_, err = stmt.ExecContext(ctx,
			transfer.ID,
			transfer.RouteID,
			transfer.FromWaypointID,
			transfer.ToWaypointID,
			transfer.DurationHours,
			transfer.DurationMinutes,
			transfer.TransportType,
			transfer.OrderIndex,
			transfer.CreatedAt,
		)
		if err != nil {
			r.log.Error("TransferRepository.CreateBatch: ошибка вставки",
				"error", err,
				"transfer_id", transfer.ID,
			)
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("TransferRepository.CreateBatch: ошибка коммита", "error", err)
		return err
	}

	r.log.Info("TransferRepository.CreateBatch: перемещения созданы", "count", len(transfers))

	return nil
}

func (r *TransferRepository) GetByRouteID(ctx context.Context, routeID uuid.UUID) ([]models.Transfer, error) {
	r.log.Debug("TransferRepository.GetByRouteID: начало", "route_id", routeID)

	query := fmt.Sprintf(`
		SELECT id, route_id, from_waypoint_id, to_waypoint_id, 
			duration_hours, duration_minutes, transport_type, order_index, created_at, updated_at
		FROM %s 
		WHERE route_id = $1
		ORDER BY order_index
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, routeID)
	if err != nil {
		r.log.Error("TransferRepository.GetByRouteID: ошибка запроса", "error", err)
		return nil, err
	}
	defer rows.Close()

	var transfers []models.Transfer
	for rows.Next() {
		var transfer models.Transfer
		var updatedAt sql.NullTime

		err := rows.Scan(
			&transfer.ID,
			&transfer.RouteID,
			&transfer.FromWaypointID,
			&transfer.ToWaypointID,
			&transfer.DurationHours,
			&transfer.DurationMinutes,
			&transfer.TransportType,
			&transfer.OrderIndex,
			&transfer.CreatedAt,
			&updatedAt,
		)
		if err != nil {
			r.log.Error("TransferRepository.GetByRouteID: ошибка сканирования", "error", err)
			return nil, err
		}

		if updatedAt.Valid {
			transfer.UpdatedAt = &updatedAt.Time
		}

		transfers = append(transfers, transfer)
	}

	r.log.Debug("TransferRepository.GetByRouteID: перемещения получены",
		"route_id", routeID,
		"count", len(transfers),
	)

	return transfers, nil
}

func (r *TransferRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Transfer, error) {
	r.log.Debug("TransferRepository.GetByID: начало", "id", id)

	query := fmt.Sprintf(`
		SELECT id, route_id, from_waypoint_id, to_waypoint_id, 
			duration_hours, duration_minutes, transport_type, order_index, created_at, updated_at
		FROM %s 
		WHERE id = $1
	`, r.table())

	var transfer models.Transfer
	var updatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&transfer.ID,
		&transfer.RouteID,
		&transfer.FromWaypointID,
		&transfer.ToWaypointID,
		&transfer.DurationHours,
		&transfer.DurationMinutes,
		&transfer.TransportType,
		&transfer.OrderIndex,
		&transfer.CreatedAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Debug("TransferRepository.GetByID: перемещение не найдено", "id", id)
			return nil, nil
		}
		r.log.Error("TransferRepository.GetByID: ошибка", "error", err)
		return nil, err
	}

	if updatedAt.Valid {
		transfer.UpdatedAt = &updatedAt.Time
	}

	return &transfer, nil
}

func (r *TransferRepository) Update(ctx context.Context, transfer *models.Transfer) error {
	r.log.Debug("TransferRepository.Update: начало", "id", transfer.ID)

	query := fmt.Sprintf(`
		UPDATE %s SET
			from_waypoint_id = $1,
			to_waypoint_id = $2,
			duration_hours = $3,
			duration_minutes = $4,
			transport_type = $5,
			order_index = $6,
			updated_at = NOW()
		WHERE id = $7
	`, r.table())

	result, err := r.db.ExecContext(ctx, query,
		transfer.FromWaypointID,
		transfer.ToWaypointID,
		transfer.DurationHours,
		transfer.DurationMinutes,
		transfer.TransportType,
		transfer.OrderIndex,
		transfer.ID,
	)
	if err != nil {
		r.log.Error("TransferRepository.Update: ошибка обновления", "error", err)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("TransferRepository.Update: перемещение обновлено",
		"id", transfer.ID,
		"rows_affected", rows,
	)

	return nil
}

func (r *TransferRepository) DeleteByRouteID(ctx context.Context, routeID uuid.UUID) error {
	r.log.Debug("TransferRepository.DeleteByRouteID: начало", "route_id", routeID)

	query := fmt.Sprintf("DELETE FROM %s WHERE route_id = $1", r.table())

	result, err := r.db.ExecContext(ctx, query, routeID)
	if err != nil {
		r.log.Error("TransferRepository.DeleteByRouteID: ошибка удаления", "error", err)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("TransferRepository.DeleteByRouteID: перемещения удалены",
		"route_id", routeID,
		"rows_affected", rows,
	)

	return nil
}

func (r *TransferRepository) Delete(ctx context.Context, id uuid.UUID) error {
	r.log.Debug("TransferRepository.Delete: начало", "id", id)

	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.table())

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error("TransferRepository.Delete: ошибка удаления", "error", err)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("TransferRepository.Delete: перемещение удалено",
		"id", id,
		"rows_affected", rows,
	)

	return nil
}

func (r *TransferRepository) ReplaceForRoute(ctx context.Context, routeID uuid.UUID, transfers []models.Transfer) error {
	r.log.Debug("TransferRepository.ReplaceForRoute: начало",
		"route_id", routeID,
		"count", len(transfers),
	)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("TransferRepository.ReplaceForRoute: ошибка начала транзакции", "error", err)
		return err
	}
	defer tx.Rollback()

	// Удаляем старые перемещения
	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE route_id = $1", r.table())
	_, err = tx.ExecContext(ctx, deleteQuery, routeID)
	if err != nil {
		r.log.Error("TransferRepository.ReplaceForRoute: ошибка удаления старых", "error", err)
		return err
	}

	// Вставляем новые
	if len(transfers) > 0 {
		insertQuery := fmt.Sprintf(`
			INSERT INTO %s (id, route_id, from_waypoint_id, to_waypoint_id, 
				duration_hours, duration_minutes, transport_type, order_index, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, r.table())

		for _, transfer := range transfers {
			_, err = tx.ExecContext(ctx, insertQuery,
				transfer.ID,
				transfer.RouteID,
				transfer.FromWaypointID,
				transfer.ToWaypointID,
				transfer.DurationHours,
				transfer.DurationMinutes,
				transfer.TransportType,
				transfer.OrderIndex,
				transfer.CreatedAt,
			)
			if err != nil {
				r.log.Error("TransferRepository.ReplaceForRoute: ошибка вставки",
					"error", err,
					"transfer_id", transfer.ID,
				)
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("TransferRepository.ReplaceForRoute: ошибка коммита", "error", err)
		return err
	}

	r.log.Info("TransferRepository.ReplaceForRoute: перемещения заменены",
		"route_id", routeID,
		"count", len(transfers),
	)

	return nil
}
