package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type NotificationRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewNotificationRepository(db *sql.DB, schema string, log *logger.Logger) *NotificationRepository {
	return &NotificationRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *NotificationRepository) table() string {
	return fmt.Sprintf("%s.notifications", r.schema)
}

func (r *NotificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	r.log.Debug("NotificationRepository.Create: начало",
		"user_id", notification.UserID,
		"type", notification.Type,
	)

	dataJSON, err := json.Marshal(notification.Data)
	if err != nil {
		r.log.Error("NotificationRepository.Create: ошибка маршалинга data", "error", err)
		return err
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (id, user_id, type, title, body, data, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, r.table())

	_, err = r.db.ExecContext(ctx, query,
		notification.ID,
		notification.UserID,
		notification.Type,
		notification.Title,
		notification.Body,
		dataJSON,
		notification.IsRead,
		notification.CreatedAt,
	)
	if err != nil {
		r.log.Error("NotificationRepository.Create: ошибка создания", "error", err)
		return err
	}

	r.log.Debug("NotificationRepository.Create: уведомление создано",
		"notification_id", notification.ID,
		"user_id", notification.UserID,
		"type", notification.Type,
	)
	return nil
}

func (r *NotificationRepository) CreateBatch(ctx context.Context, notifications []*models.Notification) error {
	if len(notifications) == 0 {
		r.log.Debug("NotificationRepository.CreateBatch: пустой список, пропускаем")
		return nil
	}

	r.log.Debug("NotificationRepository.CreateBatch: начало",
		"count", len(notifications),
	)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("NotificationRepository.CreateBatch: ошибка начала транзакции", "error", err)
		return err
	}
	defer tx.Rollback()

	query := fmt.Sprintf(`
		INSERT INTO %s (id, user_id, type, title, body, data, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, r.table())

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		r.log.Error("NotificationRepository.CreateBatch: ошибка подготовки запроса", "error", err)
		return err
	}
	defer stmt.Close()

	successCount := 0
	for _, notification := range notifications {
		dataJSON, err := json.Marshal(notification.Data)
		if err != nil {
			r.log.Error("NotificationRepository.CreateBatch: ошибка маршалинга data",
				"error", err,
				"notification_id", notification.ID,
			)
			continue
		}

		_, err = stmt.ExecContext(ctx,
			notification.ID,
			notification.UserID,
			notification.Type,
			notification.Title,
			notification.Body,
			dataJSON,
			notification.IsRead,
			notification.CreatedAt,
		)
		if err != nil {
			r.log.Error("NotificationRepository.CreateBatch: ошибка вставки",
				"error", err,
				"notification_id", notification.ID,
				"user_id", notification.UserID,
			)
			continue
		}
		successCount++
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("NotificationRepository.CreateBatch: ошибка коммита", "error", err)
		return err
	}

	r.log.Info("NotificationRepository.CreateBatch: уведомления созданы",
		"total", len(notifications),
		"success", successCount,
	)
	return nil
}

func (r *NotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Notification, error) {
	r.log.Debug("NotificationRepository.GetByID: начало", "id", id)

	query := fmt.Sprintf(`
		SELECT id, user_id, type, title, body, data, is_read, created_at
		FROM %s WHERE id = $1
	`, r.table())

	notification := &models.Notification{}
	var dataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.Type,
		&notification.Title,
		&notification.Body,
		&dataJSON,
		&notification.IsRead,
		&notification.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Debug("NotificationRepository.GetByID: уведомление не найдено", "id", id)
			return nil, nil
		}
		r.log.Error("NotificationRepository.GetByID: ошибка", "error", err, "id", id)
		return nil, err
	}

	if len(dataJSON) > 0 {
		if err := json.Unmarshal(dataJSON, &notification.Data); err != nil {
			r.log.Error("NotificationRepository.GetByID: ошибка анмаршалинга data", "error", err)
			notification.Data = make(models.JSONMap)
		}
	} else {
		notification.Data = make(models.JSONMap)
	}

	r.log.Debug("NotificationRepository.GetByID: уведомление получено", "id", id)
	return notification, nil
}

func (r *NotificationRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Notification, int64, error) {
	r.log.Debug("NotificationRepository.GetByUserID: начало",
		"user_id", userID,
		"limit", limit,
		"offset", offset,
	)

	// Получаем общее количество
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE user_id = $1", r.table())
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)
	if err != nil {
		r.log.Error("NotificationRepository.GetByUserID: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	// Получаем уведомления
	query := fmt.Sprintf(`
		SELECT id, user_id, type, title, body, data, is_read, created_at
		FROM %s 
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		r.log.Error("NotificationRepository.GetByUserID: ошибка запроса", "error", err)
		return nil, 0, err
	}
	defer rows.Close()

	var notifications []*models.Notification
	for rows.Next() {
		notification := &models.Notification{}
		var dataJSON []byte

		err := rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.Type,
			&notification.Title,
			&notification.Body,
			&dataJSON,
			&notification.IsRead,
			&notification.CreatedAt,
		)
		if err != nil {
			r.log.Error("NotificationRepository.GetByUserID: ошибка сканирования", "error", err)
			continue
		}

		if len(dataJSON) > 0 {
			if err := json.Unmarshal(dataJSON, &notification.Data); err != nil {
				notification.Data = make(models.JSONMap)
			}
		} else {
			notification.Data = make(models.JSONMap)
		}

		notifications = append(notifications, notification)
	}

	r.log.Debug("NotificationRepository.GetByUserID: уведомления получены",
		"user_id", userID,
		"count", len(notifications),
		"total", total,
	)
	return notifications, total, nil
}

func (r *NotificationRepository) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	r.log.Debug("NotificationRepository.GetUnreadCount: начало", "user_id", userID)

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s 
		WHERE user_id = $1 AND is_read = false
	`, r.table())

	var count int64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		r.log.Error("NotificationRepository.GetUnreadCount: ошибка", "error", err)
		return 0, err
	}

	r.log.Debug("NotificationRepository.GetUnreadCount: получено",
		"user_id", userID,
		"count", count,
	)
	return count, nil
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	r.log.Debug("NotificationRepository.MarkAsRead: начало",
		"id", id,
		"user_id", userID,
	)

	query := fmt.Sprintf(`
		UPDATE %s SET is_read = true 
		WHERE id = $1 AND user_id = $2
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		r.log.Error("NotificationRepository.MarkAsRead: ошибка", "error", err)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("NotificationRepository.MarkAsRead: выполнено",
		"id", id,
		"user_id", userID,
		"rows_affected", rows,
	)
	return nil
}

func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	r.log.Debug("NotificationRepository.MarkAllAsRead: начало", "user_id", userID)

	query := fmt.Sprintf(`
		UPDATE %s SET is_read = true 
		WHERE user_id = $1 AND is_read = false
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		r.log.Error("NotificationRepository.MarkAllAsRead: ошибка", "error", err)
		return 0, err
	}

	rows, _ := result.RowsAffected()
	r.log.Info("NotificationRepository.MarkAllAsRead: выполнено",
		"user_id", userID,
		"rows_affected", rows,
	)
	return rows, nil
}

func (r *NotificationRepository) DeleteOlderThan(ctx context.Context, days int) (int64, error) {
	r.log.Debug("NotificationRepository.DeleteOlderThan: начало", "days", days)

	query := fmt.Sprintf(`
		DELETE FROM %s 
		WHERE created_at < NOW() - INTERVAL '%d days'
	`, r.table(), days)

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		r.log.Error("NotificationRepository.DeleteOlderThan: ошибка", "error", err)
		return 0, err
	}

	rows, _ := result.RowsAffected()
	r.log.Info("NotificationRepository.DeleteOlderThan: старые уведомления удалены",
		"days", days,
		"rows_deleted", rows,
	)
	return rows, nil
}
