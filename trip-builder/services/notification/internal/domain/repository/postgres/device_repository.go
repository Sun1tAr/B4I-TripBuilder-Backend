package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type DeviceRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewDeviceRepository(db *sql.DB, schema string, log *logger.Logger) *DeviceRepository {
	return &DeviceRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *DeviceRepository) table() string {
	return fmt.Sprintf("%s.user_devices", r.schema)
}

func (r *DeviceRepository) Register(ctx context.Context, device *models.UserDevice) error {
	r.log.Debug("DeviceRepository.Register: начало",
		"user_id", device.UserID,
		"platform", device.Platform,
	)

	query := fmt.Sprintf(`
		INSERT INTO %s (id, user_id, token, platform, last_used)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, token) DO UPDATE SET
			last_used = EXCLUDED.last_used,
			platform = EXCLUDED.platform
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		device.ID,
		device.UserID,
		device.Token,
		device.Platform,
		device.LastUsed,
	)
	if err != nil {
		r.log.Error("DeviceRepository.Register: ошибка регистрации", "error", err)
		return err
	}

	r.log.Debug("DeviceRepository.Register: устройство зарегистрировано",
		"device_id", device.ID,
		"user_id", device.UserID,
		"platform", device.Platform,
	)
	return nil
}

func (r *DeviceRepository) Unregister(ctx context.Context, userID uuid.UUID, token string) error {
	r.log.Debug("DeviceRepository.Unregister: начало",
		"user_id", userID,
	)

	query := fmt.Sprintf(`
		DELETE FROM %s WHERE user_id = $1 AND token = $2
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, userID, token)
	if err != nil {
		r.log.Error("DeviceRepository.Unregister: ошибка удаления", "error", err)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("DeviceRepository.Unregister: устройство удалено",
		"user_id", userID,
		"rows_affected", rows,
	)
	return nil
}

func (r *DeviceRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.UserDevice, error) {
	r.log.Debug("DeviceRepository.GetByUserID: начало", "user_id", userID)

	query := fmt.Sprintf(`
		SELECT id, user_id, token, platform, last_used
		FROM %s WHERE user_id = $1
		ORDER BY last_used DESC
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		r.log.Error("DeviceRepository.GetByUserID: ошибка запроса", "error", err)
		return nil, err
	}
	defer rows.Close()

	var devices []*models.UserDevice
	for rows.Next() {
		device := &models.UserDevice{}
		err := rows.Scan(
			&device.ID,
			&device.UserID,
			&device.Token,
			&device.Platform,
			&device.LastUsed,
		)
		if err != nil {
			r.log.Error("DeviceRepository.GetByUserID: ошибка сканирования", "error", err)
			continue
		}
		devices = append(devices, device)
	}

	r.log.Debug("DeviceRepository.GetByUserID: устройства получены",
		"user_id", userID,
		"count", len(devices),
	)
	return devices, nil
}

func (r *DeviceRepository) GetByToken(ctx context.Context, token string) (*models.UserDevice, error) {
	r.log.Debug("DeviceRepository.GetByToken: начало")

	query := fmt.Sprintf(`
		SELECT id, user_id, token, platform, last_used
		FROM %s WHERE token = $1
	`, r.table())

	device := &models.UserDevice{}
	err := r.db.QueryRowContext(ctx, query, token).Scan(
		&device.ID,
		&device.UserID,
		&device.Token,
		&device.Platform,
		&device.LastUsed,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Debug("DeviceRepository.GetByToken: устройство не найдено")
			return nil, nil
		}
		r.log.Error("DeviceRepository.GetByToken: ошибка", "error", err)
		return nil, err
	}

	return device, nil
}

func (r *DeviceRepository) UpdateLastUsed(ctx context.Context, userID uuid.UUID, token string) error {
	r.log.Debug("DeviceRepository.UpdateLastUsed: начало", "user_id", userID)

	query := fmt.Sprintf(`
		UPDATE %s SET last_used = NOW()
		WHERE user_id = $1 AND token = $2
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, userID, token)
	if err != nil {
		r.log.Error("DeviceRepository.UpdateLastUsed: ошибка", "error", err)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("DeviceRepository.UpdateLastUsed: обновлено",
		"user_id", userID,
		"rows_affected", rows,
	)
	return nil
}

func (r *DeviceRepository) DeleteInactive(ctx context.Context, days int) (int64, error) {
	r.log.Debug("DeviceRepository.DeleteInactive: начало", "days", days)

	query := fmt.Sprintf(`
		DELETE FROM %s 
		WHERE last_used < NOW() - INTERVAL '%d days'
	`, r.table(), days)

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		r.log.Error("DeviceRepository.DeleteInactive: ошибка", "error", err)
		return 0, err
	}

	rows, _ := result.RowsAffected()
	r.log.Info("DeviceRepository.DeleteInactive: неактивные устройства удалены",
		"days", days,
		"rows_deleted", rows,
	)
	return rows, nil
}
