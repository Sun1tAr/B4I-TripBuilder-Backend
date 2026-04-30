package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type LikeRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewLikeRepository(db *sql.DB, schema string, log *logger.Logger) *LikeRepository {
	return &LikeRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *LikeRepository) table() string {
	return fmt.Sprintf("%s.route_likes", r.schema)
}

func (r *LikeRepository) routesTable() string {
	return fmt.Sprintf("%s.routes", r.schema)
}

func (r *LikeRepository) Like(ctx context.Context, userID, routeID uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("LikeRepository.Like: ошибка начала транзакции", "error", err)
		return err
	}
	defer tx.Rollback()

	// Вставляем лайк
	insertQuery := fmt.Sprintf(`
		INSERT INTO %s (user_id, route_id, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id, route_id) DO NOTHING
	`, r.table())

	result, err := tx.ExecContext(ctx, insertQuery, userID, routeID)
	if err != nil {
		r.log.Error("LikeRepository.Like: ошибка вставки", "error", err)
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Уже лайкнуто
		return nil
	}

	// Увеличиваем счётчик лайков в маршруте
	updateQuery := fmt.Sprintf("UPDATE %s SET likes_count = likes_count + 1 WHERE id = $1", r.routesTable())
	_, err = tx.ExecContext(ctx, updateQuery, routeID)
	if err != nil {
		r.log.Error("LikeRepository.Like: ошибка обновления счётчика", "error", err)
		return err
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("LikeRepository.Like: ошибка коммита", "error", err)
		return err
	}

	r.log.Debug("LikeRepository.Like: лайк поставлен", "user_id", userID, "route_id", routeID)
	return nil
}

func (r *LikeRepository) Unlike(ctx context.Context, userID, routeID uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("LikeRepository.Unlike: ошибка начала транзакции", "error", err)
		return err
	}
	defer tx.Rollback()

	// Удаляем лайк
	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE user_id = $1 AND route_id = $2", r.table())
	result, err := tx.ExecContext(ctx, deleteQuery, userID, routeID)
	if err != nil {
		r.log.Error("LikeRepository.Unlike: ошибка удаления", "error", err)
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Лайка не было
		return nil
	}

	// Уменьшаем счётчик лайков в маршруте
	updateQuery := fmt.Sprintf("UPDATE %s SET likes_count = GREATEST(0, likes_count - 1) WHERE id = $1", r.routesTable())
	_, err = tx.ExecContext(ctx, updateQuery, routeID)
	if err != nil {
		r.log.Error("LikeRepository.Unlike: ошибка обновления счётчика", "error", err)
		return err
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("LikeRepository.Unlike: ошибка коммита", "error", err)
		return err
	}

	r.log.Debug("LikeRepository.Unlike: лайк убран", "user_id", userID, "route_id", routeID)
	return nil
}

func (r *LikeRepository) IsLiked(ctx context.Context, userID, routeID uuid.UUID) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE user_id = $1 AND route_id = $2)", r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID, routeID).Scan(&exists)
	if err != nil {
		r.log.Error("LikeRepository.IsLiked: ошибка проверки", "error", err)
		return false, err
	}
	return exists, nil
}

func (r *LikeRepository) CountByRouteID(ctx context.Context, routeID uuid.UUID) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE route_id = $1", r.table())
	var count int
	err := r.db.QueryRowContext(ctx, query, routeID).Scan(&count)
	if err != nil {
		r.log.Error("LikeRepository.CountByRouteID: ошибка подсчёта", "error", err)
		return 0, err
	}
	return count, nil
}

func (r *LikeRepository) GetUsersByRouteID(ctx context.Context, routeID uuid.UUID, limit, offset int) ([]uuid.UUID, int64, error) {
	query := fmt.Sprintf(`
		SELECT user_id FROM %s 
		WHERE route_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, routeID, limit, offset)
	if err != nil {
		r.log.Error("LikeRepository.GetUsersByRouteID: ошибка запроса", "error", err)
		return nil, 0, err
	}
	defer rows.Close()

	var userIDs []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			r.log.Error("LikeRepository.GetUsersByRouteID: ошибка сканирования", "error", err)
			return nil, 0, err
		}
		userIDs = append(userIDs, userID)
	}

	// Получаем общее количество
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE route_id = $1", r.table())
	var total int64
	err = r.db.QueryRowContext(ctx, countQuery, routeID).Scan(&total)
	if err != nil {
		r.log.Error("LikeRepository.GetUsersByRouteID: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	return userIDs, total, nil
}

// GetLikedRoutesByUserID возвращает ID маршрутов, лайкнутых пользователем
func (r *LikeRepository) GetLikedRoutesByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]uuid.UUID, int64, error) {
	r.log.Debug("LikeRepository.GetLikedRoutesByUserID: начало",
		"user_id", userID,
		"limit", limit,
		"offset", offset,
	)

	// Получаем общее количество
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE user_id = $1", r.table())
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)
	if err != nil {
		r.log.Error("LikeRepository.GetLikedRoutesByUserID: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	// Получаем ID маршрутов
	query := fmt.Sprintf(`
		SELECT route_id FROM %s 
		WHERE user_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		r.log.Error("LikeRepository.GetLikedRoutesByUserID: ошибка запроса", "error", err)
		return nil, 0, err
	}
	defer rows.Close()

	var routeIDs []uuid.UUID
	for rows.Next() {
		var routeID uuid.UUID
		if err := rows.Scan(&routeID); err != nil {
			r.log.Error("LikeRepository.GetLikedRoutesByUserID: ошибка сканирования", "error", err)
			return nil, 0, err
		}
		routeIDs = append(routeIDs, routeID)
	}

	r.log.Debug("LikeRepository.GetLikedRoutesByUserID: получено",
		"user_id", userID,
		"count", len(routeIDs),
		"total", total,
	)

	return routeIDs, total, nil
}