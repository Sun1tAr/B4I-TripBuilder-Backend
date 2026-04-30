package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/domain/models"
	"github.com/google/uuid"
)

type UserRepository struct {
	db     *sql.DB
	schema string
}

func NewUserRepository(db *sql.DB, schema string) *UserRepository {
	return &UserRepository{db: db, schema: schema}
}

func (r *UserRepository) table() string {
	return fmt.Sprintf("%s.users", r.schema)
}

func (r *UserRepository) Create(ctx context.Context, email, passwordHash string) (*models.User, error) {
	query := fmt.Sprintf(`
		INSERT INTO %s (email, password_hash, is_verified, created_at)
		VALUES ($1, $2, false, NOW())
		RETURNING id, email, password_hash, is_verified, created_at, last_login
	`, r.table())

	user := &models.User{}
	err := r.db.QueryRowContext(ctx, query, email, passwordHash).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.IsVerified, &user.CreatedAt, &user.LastLogin,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := fmt.Sprintf(`
		SELECT id, email, password_hash, is_verified, created_at, last_login
		FROM %s WHERE email = $1
	`, r.table())

	user := &models.User{}
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.IsVerified, &user.CreatedAt, &user.LastLogin,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := fmt.Sprintf(`
		SELECT id, email, password_hash, is_verified, created_at, last_login
		FROM %s WHERE id = $1
	`, r.table())

	user := &models.User{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.IsVerified, &user.CreatedAt, &user.LastLogin,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf(`
		UPDATE %s SET last_login = NOW() WHERE id = $1
	`, r.table())
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *UserRepository) VerifyEmail(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf(`
		UPDATE %s SET is_verified = true WHERE id = $1
	`, r.table())
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	query := fmt.Sprintf(`
		UPDATE %s SET password_hash = $1 WHERE id = $2
	`, r.table())
	_, err := r.db.ExecContext(ctx, query, passwordHash, id)
	return err
}