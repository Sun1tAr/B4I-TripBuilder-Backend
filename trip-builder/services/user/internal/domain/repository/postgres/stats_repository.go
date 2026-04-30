package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type StatsRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewStatsRepository(db *sql.DB, schema string, log *logger.Logger) *StatsRepository {
	return &StatsRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *StatsRepository) table() string {
	return fmt.Sprintf("%s.stats", r.schema)
}

func (r *StatsRepository) Create(ctx context.Context, userID uuid.UUID) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (user_id, routes_created, events_done, countries_visited, photos_uploaded, followers, following)
		VALUES ($1, 0, 0, 0, 0, 0, 0)
	`, r.table())

	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		r.log.Error("StatsRepository.Create: ошибка создания статистики", "error", err, "user_id", userID)
		return err
	}

	r.log.Debug("StatsRepository.Create: статистика создана", "user_id", userID)
	return nil
}

func (r *StatsRepository) GetByID(ctx context.Context, userID uuid.UUID) (*models.Stats, error) {
	query := fmt.Sprintf(`
		SELECT user_id, routes_created, events_done, countries_visited, photos_uploaded, followers, following
		FROM %s WHERE user_id = $1
	`, r.table())

	stats := &models.Stats{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&stats.UserID,
		&stats.RoutesCreated,
		&stats.EventsDone,
		&stats.CountriesVisited,
		&stats.PhotosUploaded,
		&stats.Followers,
		&stats.Following,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("StatsRepository.GetByID: ошибка получения статистики", "error", err, "user_id", userID)
		return nil, err
	}
	return stats, nil
}

func (r *StatsRepository) increment(ctx context.Context, userID uuid.UUID, field string) error {
	query := fmt.Sprintf(`
		UPDATE %s SET %s = %s + 1 WHERE user_id = $1
	`, r.table(), field, field)

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		r.log.Error("StatsRepository.increment: ошибка", "error", err, "user_id", userID, "field", field)
		return err
	}
	rows, _ := result.RowsAffected()
	r.log.Debug("StatsRepository.increment: обновлено", "user_id", userID, "field", field, "rows_affected", rows)
	return nil
}

func (r *StatsRepository) decrement(ctx context.Context, userID uuid.UUID, field string) error {
	query := fmt.Sprintf(`
		UPDATE %s SET %s = GREATEST(0, %s - 1) WHERE user_id = $1
	`, r.table(), field, field)

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		r.log.Error("StatsRepository.decrement: ошибка", "error", err, "user_id", userID, "field", field)
		return err
	}
	rows, _ := result.RowsAffected()
	r.log.Debug("StatsRepository.decrement: обновлено", "user_id", userID, "field", field, "rows_affected", rows)
	return nil
}

func (r *StatsRepository) IncrementRoutesCreated(ctx context.Context, userID uuid.UUID) error {
	return r.increment(ctx, userID, "routes_created")
}

func (r *StatsRepository) IncrementEventsDone(ctx context.Context, userID uuid.UUID) error {
	return r.increment(ctx, userID, "events_done")
}

func (r *StatsRepository) IncrementCountriesVisited(ctx context.Context, userID uuid.UUID, count int) error {
	query := fmt.Sprintf(`
		UPDATE %s SET countries_visited = countries_visited + $1 WHERE user_id = $2
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, count, userID)
	if err != nil {
		r.log.Error("StatsRepository.IncrementCountriesVisited: ошибка", "error", err, "user_id", userID, "count", count)
		return err
	}
	rows, _ := result.RowsAffected()
	r.log.Debug("StatsRepository.IncrementCountriesVisited: обновлено", "user_id", userID, "count", count, "rows_affected", rows)
	return nil
}

func (r *StatsRepository) IncrementPhotosUploaded(ctx context.Context, userID uuid.UUID) error {
	return r.increment(ctx, userID, "photos_uploaded")
}

func (r *StatsRepository) IncrementFollowers(ctx context.Context, userID uuid.UUID) error {
	return r.increment(ctx, userID, "followers")
}

func (r *StatsRepository) DecrementFollowers(ctx context.Context, userID uuid.UUID) error {
	return r.decrement(ctx, userID, "followers")
}

func (r *StatsRepository) IncrementFollowing(ctx context.Context, userID uuid.UUID) error {
	return r.increment(ctx, userID, "following")
}

func (r *StatsRepository) DecrementFollowing(ctx context.Context, userID uuid.UUID) error {
	return r.decrement(ctx, userID, "following")
}