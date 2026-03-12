package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type RefreshTokenRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewRefreshTokenRepository(db *sql.DB, schema string, log *logger.Logger) *RefreshTokenRepository {
	return &RefreshTokenRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *RefreshTokenRepository) table() string {
	return fmt.Sprintf("%s.refresh_tokens", r.schema)
}

func (r *RefreshTokenRepository) Create(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) (*models.RefreshToken, error) {
	query := fmt.Sprintf(`
		INSERT INTO %s (user_id, token, expires_at, revoked, created_at)
		VALUES ($1, $2, $3, false, NOW())
		RETURNING id, user_id, token, expires_at, revoked, created_at
	`, r.table())

	rt := &models.RefreshToken{}
	err := r.db.QueryRowContext(ctx, query, userID, token, expiresAt).Scan(
		&rt.ID, &rt.UserID, &rt.Token, &rt.ExpiresAt, &rt.Revoked, &rt.CreatedAt,
	)
	if err != nil {
		r.log.Error("Ошибка создания refresh токена", "error", err, "user_id", userID)
		return nil, err
	}
	return rt, nil
}

func (r *RefreshTokenRepository) GetByToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	query := fmt.Sprintf(`
		SELECT id, user_id, token, expires_at, revoked, created_at
		FROM %s WHERE token = $1
	`, r.table())

	rt := &models.RefreshToken{}
	err := r.db.QueryRowContext(ctx, query, token).Scan(
		&rt.ID, &rt.UserID, &rt.Token, &rt.ExpiresAt, &rt.Revoked, &rt.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Debug("RefreshTokenRepo: токен не найден", "token", token)
			return nil, nil
		}
		r.log.Error("RefreshTokenRepo: ошибка БД", "error", err, "token", token)
		return nil, err
	}


	return rt, nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, token string) error {
	query := fmt.Sprintf(`
		UPDATE %s SET revoked = true WHERE token = $1
	`, r.table())
	result, err := r.db.ExecContext(ctx, query, token)
	if err != nil {
		r.log.Error("RefreshTokenRepo: ошибка отзыва токена", "error", err, "token", token)
		return err
	}
	_, _ = result.RowsAffected()
	return nil
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	query := fmt.Sprintf(`
		UPDATE %s SET revoked = true WHERE user_id = $1 AND revoked = false
	`, r.table())
	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		r.log.Error("RefreshTokenRepo: ошибка отзыва всех токенов", "error", err, "user_id", userID)
		return err
	}
	_, _ = result.RowsAffected()
	return nil
}

func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context) error {
	query := fmt.Sprintf(`
		DELETE FROM %s WHERE expires_at < NOW()
	`, r.table())
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		r.log.Error("RefreshTokenRepo: ошибка удаления просроченных токенов", "error", err)
		return err
	}
	rows, _ := result.RowsAffected()
	r.log.Debug("RefreshTokenRepo: удалены просроченные токены", "rows_deleted", rows)
	return nil
}