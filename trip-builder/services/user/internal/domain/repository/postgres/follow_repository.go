package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type FollowRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewFollowRepository(db *sql.DB, schema string, log *logger.Logger) *FollowRepository {
	return &FollowRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *FollowRepository) table() string {
	return fmt.Sprintf("%s.follows", r.schema)
}

func (r *FollowRepository) Follow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (follower_id, followee_id, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (follower_id, followee_id) DO NOTHING
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, followerID, followeeID)
	if err != nil {
		r.log.Error("FollowRepository.Follow: ошибка подписки",
			"error", err, "follower_id", followerID, "followee_id", followeeID)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("FollowRepository.Follow: подписка оформлена",
		"follower_id", followerID, "followee_id", followeeID, "rows_affected", rows)
	return nil
}

func (r *FollowRepository) Unfollow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	query := fmt.Sprintf(`
		DELETE FROM %s WHERE follower_id = $1 AND followee_id = $2
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, followerID, followeeID)
	if err != nil {
		r.log.Error("FollowRepository.Unfollow: ошибка отписки",
			"error", err, "follower_id", followerID, "followee_id", followeeID)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("FollowRepository.Unfollow: отписка выполнена",
		"follower_id", followerID, "followee_id", followeeID, "rows_affected", rows)
	return nil
}

func (r *FollowRepository) IsFollowing(ctx context.Context, followerID, followeeID uuid.UUID) (bool, error) {
	query := fmt.Sprintf(`
		SELECT EXISTS(SELECT 1 FROM %s WHERE follower_id = $1 AND followee_id = $2)
	`, r.table())

	var exists bool
	err := r.db.QueryRowContext(ctx, query, followerID, followeeID).Scan(&exists)
	if err != nil {
		r.log.Error("FollowRepository.IsFollowing: ошибка проверки",
			"error", err, "follower_id", followerID, "followee_id", followeeID)
		return false, err
	}
	return exists, nil
}

func (r *FollowRepository) GetFollowers(ctx context.Context, userID uuid.UUID, limit, offset int) ([]uuid.UUID, error) {
	query := fmt.Sprintf(`
		SELECT follower_id FROM %s WHERE followee_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		r.log.Error("FollowRepository.GetFollowers: ошибка запроса", "error", err, "user_id", userID)
		return nil, err
	}
	defer rows.Close()

	var followers []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			r.log.Error("FollowRepository.GetFollowers: ошибка сканирования", "error", err)
			return nil, err
		}
		followers = append(followers, id)
	}
	return followers, nil
}

func (r *FollowRepository) GetFollowing(ctx context.Context, userID uuid.UUID, limit, offset int) ([]uuid.UUID, error) {
	query := fmt.Sprintf(`
		SELECT followee_id FROM %s WHERE follower_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		r.log.Error("FollowRepository.GetFollowing: ошибка запроса", "error", err, "user_id", userID)
		return nil, err
	}
	defer rows.Close()

	var following []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			r.log.Error("FollowRepository.GetFollowing: ошибка сканирования", "error", err)
			return nil, err
		}
		following = append(following, id)
	}
	return following, nil
}

func (r *FollowRepository) CountFollowers(ctx context.Context, userID uuid.UUID) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE followee_id = $1`, r.table())

	var count int
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		r.log.Error("FollowRepository.CountFollowers: ошибка подсчёта", "error", err, "user_id", userID)
		return 0, err
	}
	return count, nil
}

func (r *FollowRepository) CountFollowing(ctx context.Context, userID uuid.UUID) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE follower_id = $1`, r.table())

	var count int
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		r.log.Error("FollowRepository.CountFollowing: ошибка подсчёта", "error", err, "user_id", userID)
		return 0, err
	}
	return count, nil
}