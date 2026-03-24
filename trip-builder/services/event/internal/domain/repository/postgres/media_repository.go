package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type MediaRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewMediaRepository(db *sql.DB, schema string, log *logger.Logger) *MediaRepository {
	return &MediaRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *MediaRepository) table() string {
	return fmt.Sprintf("%s.waypoint_media", r.schema)
}

func (r *MediaRepository) Create(ctx context.Context, media *models.Media) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (id, event_id, waypoint_id, user_id, media_url, thumbnail_url, medium_url, media_type, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		media.ID, media.EventID, media.WaypointID, media.UserID,
		media.MediaURL, media.ThumbnailURL, media.MediumURL,
		media.MediaType, media.Description, media.CreatedAt,
	)
	if err != nil {
		r.log.Error("MediaRepository.Create: ошибка создания", "error", err, "media_id", media.ID)
		return err
	}

	r.log.Debug("MediaRepository.Create: медиа создано", "media_id", media.ID)
	return nil
}

func (r *MediaRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Media, error) {
	query := fmt.Sprintf(`
		SELECT id, event_id, waypoint_id, user_id, media_url, thumbnail_url, medium_url, media_type, description, created_at
		FROM %s WHERE id = $1
	`, r.table())

	media := &models.Media{}
	var thumbnailURL, mediumURL, description sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&media.ID, &media.EventID, &media.WaypointID, &media.UserID,
		&media.MediaURL, &thumbnailURL, &mediumURL,
		&media.MediaType, &description, &media.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("MediaRepository.GetByID: ошибка", "error", err, "media_id", id)
		return nil, err
	}

	if thumbnailURL.Valid {
		media.ThumbnailURL = thumbnailURL.String
	}
	if mediumURL.Valid {
		media.MediumURL = mediumURL.String
	}
	if description.Valid {
		media.Description = description.String
	}

	return media, nil
}

func (r *MediaRepository) GetByWaypointID(ctx context.Context, waypointID uuid.UUID, limit, offset int) ([]models.Media, int64, error) {
	query := fmt.Sprintf(`
		SELECT id, event_id, waypoint_id, user_id, media_url, thumbnail_url, medium_url, media_type, description, created_at
		FROM %s WHERE waypoint_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, waypointID, limit, offset)
	if err != nil {
		r.log.Error("MediaRepository.GetByWaypointID: ошибка запроса", "error", err, "waypoint_id", waypointID)
		return nil, 0, err
	}
	defer rows.Close()

	var mediaList []models.Media
	for rows.Next() {
		var media models.Media
		var thumbnailURL, mediumURL, description sql.NullString

		err := rows.Scan(
			&media.ID, &media.EventID, &media.WaypointID, &media.UserID,
			&media.MediaURL, &thumbnailURL, &mediumURL,
			&media.MediaType, &description, &media.CreatedAt,
		)
		if err != nil {
			r.log.Error("MediaRepository.GetByWaypointID: ошибка сканирования", "error", err)
			return nil, 0, err
		}

		if thumbnailURL.Valid {
			media.ThumbnailURL = thumbnailURL.String
		}
		if mediumURL.Valid {
			media.MediumURL = mediumURL.String
		}
		if description.Valid {
			media.Description = description.String
		}

		mediaList = append(mediaList, media)
	}

	// Получаем общее количество
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE waypoint_id = $1", r.table())
	var total int64
	err = r.db.QueryRowContext(ctx, countQuery, waypointID).Scan(&total)
	if err != nil {
		r.log.Error("MediaRepository.GetByWaypointID: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	return mediaList, total, nil
}

func (r *MediaRepository) GetByEventID(ctx context.Context, eventID uuid.UUID, limit, offset int) ([]models.Media, int64, error) {
	query := fmt.Sprintf(`
		SELECT id, event_id, waypoint_id, user_id, media_url, thumbnail_url, medium_url, media_type, description, created_at
		FROM %s WHERE event_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, eventID, limit, offset)
	if err != nil {
		r.log.Error("MediaRepository.GetByEventID: ошибка запроса", "error", err, "event_id", eventID)
		return nil, 0, err
	}
	defer rows.Close()

	var mediaList []models.Media
	for rows.Next() {
		var media models.Media
		var thumbnailURL, mediumURL, description sql.NullString

		err := rows.Scan(
			&media.ID, &media.EventID, &media.WaypointID, &media.UserID,
			&media.MediaURL, &thumbnailURL, &mediumURL,
			&media.MediaType, &description, &media.CreatedAt,
		)
		if err != nil {
			r.log.Error("MediaRepository.GetByEventID: ошибка сканирования", "error", err)
			return nil, 0, err
		}

		if thumbnailURL.Valid {
			media.ThumbnailURL = thumbnailURL.String
		}
		if mediumURL.Valid {
			media.MediumURL = mediumURL.String
		}
		if description.Valid {
			media.Description = description.String
		}

		mediaList = append(mediaList, media)
	}

	// Получаем общее количество
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE event_id = $1", r.table())
	var total int64
	err = r.db.QueryRowContext(ctx, countQuery, eventID).Scan(&total)
	if err != nil {
		r.log.Error("MediaRepository.GetByEventID: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	return mediaList, total, nil
}

func (r *MediaRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.table())
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error("MediaRepository.Delete: ошибка удаления", "error", err, "media_id", id)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("MediaRepository.Delete: медиа удалено", "media_id", id, "rows", rows)
	return nil
}

func (r *MediaRepository) DeleteByWaypointID(ctx context.Context, waypointID uuid.UUID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE waypoint_id = $1", r.table())
	result, err := r.db.ExecContext(ctx, query, waypointID)
	if err != nil {
		r.log.Error("MediaRepository.DeleteByWaypointID: ошибка удаления", "error", err, "waypoint_id", waypointID)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("MediaRepository.DeleteByWaypointID: медиа удалены", "waypoint_id", waypointID, "rows", rows)
	return nil
}

func (r *MediaRepository) IsOwner(ctx context.Context, mediaID, userID uuid.UUID) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1 AND user_id = $2)", r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, mediaID, userID).Scan(&exists)
	if err != nil {
		r.log.Error("MediaRepository.IsOwner: ошибка проверки", "error", err, "media_id", mediaID, "user_id", userID)
		return false, err
	}
	return exists, nil
}
