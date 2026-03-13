package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type AchievementRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewAchievementRepository(db *sql.DB, schema string, log *logger.Logger) *AchievementRepository {
	return &AchievementRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *AchievementRepository) table() string {
	return fmt.Sprintf("%s.achievements", r.schema)
}

func (r *AchievementRepository) Create(ctx context.Context, userID uuid.UUID, achievementType string, title, description string) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (user_id, type, title, description, earned_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, r.table())

	_, err := r.db.ExecContext(ctx, query, userID, achievementType, title, description)
	if err != nil {
		r.log.Error("AchievementRepository.Create: ошибка создания достижения",
			"error", err, "user_id", userID, "type", achievementType)
		return err
	}

	r.log.Info("Достижение получено", "user_id", userID, "type", achievementType, "title", title)
	return nil
}

func (r *AchievementRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Achievement, error) {
	query := fmt.Sprintf(`
		SELECT id, user_id, type, title, description, earned_at
		FROM %s WHERE user_id = $1 ORDER BY earned_at DESC
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		r.log.Error("AchievementRepository.GetByUserID: ошибка запроса", "error", err, "user_id", userID)
		return nil, err
	}
	defer rows.Close()

	var achievements []*models.Achievement
	for rows.Next() {
		a := &models.Achievement{}
		err := rows.Scan(&a.ID, &a.UserID, &a.Type, &a.Title, &a.Description, &a.EarnedAt)
		if err != nil {
			r.log.Error("AchievementRepository.GetByUserID: ошибка сканирования", "error", err)
			return nil, err
		}
		achievements = append(achievements, a)
	}
	return achievements, nil
}

func (r *AchievementRepository) HasAchievement(ctx context.Context, userID uuid.UUID, achievementType string) (bool, error) {
	query := fmt.Sprintf(`
		SELECT EXISTS(SELECT 1 FROM %s WHERE user_id = $1 AND type = $2)
	`, r.table())

	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID, achievementType).Scan(&exists)
	if err != nil {
		r.log.Error("AchievementRepository.HasAchievement: ошибка проверки",
			"error", err, "user_id", userID, "type", achievementType)
		return false, err
	}
	return exists, nil
}