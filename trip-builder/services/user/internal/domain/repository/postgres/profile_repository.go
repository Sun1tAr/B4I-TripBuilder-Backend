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

type ProfileRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewProfileRepository(db *sql.DB, schema string, log *logger.Logger) *ProfileRepository {
	return &ProfileRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *ProfileRepository) table() string {
	return fmt.Sprintf("%s.profiles", r.schema)
}

func (r *ProfileRepository) Create(ctx context.Context, userID uuid.UUID) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (user_id, language, timezone, notifications_enabled, created_at)
		VALUES ($1, 'ru', 'UTC', true, NOW())
	`, r.table())

	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		r.log.Error("ProfileRepository.Create: ошибка создания профиля", "error", err, "user_id", userID)
		return err
	}

	r.log.Debug("ProfileRepository.Create: профиль создан", "user_id", userID)
	return nil
}

func (r *ProfileRepository) GetByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error) {
	query := fmt.Sprintf(`
		SELECT user_id, first_name, last_name, avatar_url, bio, phone, 
		       language, timezone, notifications_enabled, created_at, updated_at
		FROM %s WHERE user_id = $1
	`, r.table())

	profile := &models.Profile{}
	var firstName, lastName, avatarURL, bio, phone sql.NullString
	var updatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&profile.UserID,
		&firstName,
		&lastName,
		&avatarURL,
		&bio,
		&phone,
		&profile.Language,
		&profile.Timezone,
		&profile.NotificationsEnabled,
		&profile.CreatedAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("ProfileRepository.GetByID: ошибка получения профиля", "error", err, "user_id", userID)
		return nil, err
	}

	profile.FirstName = firstName.String
	profile.LastName = lastName.String
	profile.AvatarURL = avatarURL.String
	profile.Bio = bio.String
	profile.Phone = phone.String
	if updatedAt.Valid {
		profile.UpdatedAt = &updatedAt.Time
	}

	return profile, nil
}

func (r *ProfileRepository) Update(ctx context.Context, userID uuid.UUID, updates *models.UpdateProfileRequest) error {
	query := fmt.Sprintf(`
		UPDATE %s SET
			first_name = COALESCE($1, first_name),
			last_name = COALESCE($2, last_name),
			bio = COALESCE($3, bio),
			phone = COALESCE($4, phone),
			language = COALESCE($5, language),
			timezone = COALESCE($6, timezone),
			notifications_enabled = COALESCE($7, notifications_enabled),
			updated_at = NOW()
		WHERE user_id = $8
	`, r.table())

	result, err := r.db.ExecContext(ctx, query,
		updates.FirstName,
		updates.LastName,
		updates.Bio,
		updates.Phone,
		updates.Language,
		updates.Timezone,
		updates.NotificationsEnabled,
		userID,
	)
	if err != nil {
		r.log.Error("ProfileRepository.Update: ошибка обновления профиля", "error", err, "user_id", userID)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("ProfileRepository.Update: профиль обновлён", "user_id", userID, "rows_affected", rows)
	return nil
}

func (r *ProfileRepository) UpdateAvatar(ctx context.Context, userID uuid.UUID, avatarURL string) error {
	query := fmt.Sprintf(`
		UPDATE %s SET avatar_url = $1, updated_at = NOW() WHERE user_id = $2
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, avatarURL, userID)
	if err != nil {
		r.log.Error("ProfileRepository.UpdateAvatar: ошибка обновления аватара", "error", err, "user_id", userID)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("ProfileRepository.UpdateAvatar: аватар обновлён", "user_id", userID, "rows_affected", rows)
	return nil
}

func (r *ProfileRepository) Exists(ctx context.Context, userID uuid.UUID) (bool, error) {
	query := fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE user_id = $1)`, r.table())

	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&exists)
	if err != nil {
		r.log.Error("ProfileRepository.Exists: ошибка проверки существования", "error", err, "user_id", userID)
		return false, err
	}
	return exists, nil
}