package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type MessageRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewMessageRepository(db *sql.DB, schema string, log *logger.Logger) *MessageRepository {
	return &MessageRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *MessageRepository) table() string {
	return fmt.Sprintf("%s.messages", r.schema)
}

func (r *MessageRepository) Create(ctx context.Context, msg *models.Message) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (id, event_id, user_id, text, type, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, r.table())

	metadataJSON, err := json.Marshal(msg.Metadata)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, query,
		msg.ID, msg.EventID, msg.UserID, msg.Text, msg.Type,
		metadataJSON, msg.CreatedAt,
	)
	if err != nil {
		r.log.Error("MessageRepository.Create: ошибка создания", "error", err, "message_id", msg.ID)
		return err
	}

	r.log.Debug("MessageRepository.Create: сообщение создано", "message_id", msg.ID)
	return nil
}

func (r *MessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	query := fmt.Sprintf(`
		SELECT id, event_id, user_id, text, type, metadata, created_at, updated_at
		FROM %s WHERE id = $1
	`, r.table())

	msg := &models.Message{}
	var metadataJSON []byte
	var updatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&msg.ID, &msg.EventID, &msg.UserID, &msg.Text, &msg.Type,
		&metadataJSON, &msg.CreatedAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("MessageRepository.GetByID: ошибка", "error", err, "message_id", id)
		return nil, err
	}

	if len(metadataJSON) > 0 {
		var metadata models.MessageMetadata
		if err := json.Unmarshal(metadataJSON, &metadata); err == nil {
			msg.Metadata = metadata
		}
	}
	if updatedAt.Valid {
		msg.UpdatedAt = &updatedAt.Time
	}

	return msg, nil
}

func (r *MessageRepository) GetByEventID(ctx context.Context, eventID uuid.UUID, limit, offset int) ([]models.Message, int64, error) {
	query := fmt.Sprintf(`
		SELECT id, event_id, user_id, text, type, metadata, created_at, updated_at
		FROM %s WHERE event_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, eventID, limit, offset)
	if err != nil {
		r.log.Error("MessageRepository.GetByEventID: ошибка запроса", "error", err, "event_id", eventID)
		return nil, 0, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		var metadataJSON []byte
		var updatedAt sql.NullTime

		err := rows.Scan(
			&msg.ID, &msg.EventID, &msg.UserID, &msg.Text, &msg.Type,
			&metadataJSON, &msg.CreatedAt, &updatedAt,
		)
		if err != nil {
			r.log.Error("MessageRepository.GetByEventID: ошибка сканирования", "error", err)
			return nil, 0, err
		}

		if len(metadataJSON) > 0 {
			var metadata models.MessageMetadata
			if err := json.Unmarshal(metadataJSON, &metadata); err == nil {
				msg.Metadata = metadata
			}
		}
		if updatedAt.Valid {
			msg.UpdatedAt = &updatedAt.Time
		}

		messages = append(messages, msg)
	}

	// Получаем общее количество
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE event_id = $1", r.table())
	var total int64
	err = r.db.QueryRowContext(ctx, countQuery, eventID).Scan(&total)
	if err != nil {
		r.log.Error("MessageRepository.GetByEventID: ошибка подсчёта", "error", err)
		return nil, 0, err
	}

	return messages, total, nil
}

func (r *MessageRepository) Update(ctx context.Context, msg *models.Message) error {
	query := fmt.Sprintf(`
		UPDATE %s SET text = $1, updated_at = NOW()
		WHERE id = $2
	`, r.table())

	_, err := r.db.ExecContext(ctx, query, msg.Text, msg.ID)
	if err != nil {
		r.log.Error("MessageRepository.Update: ошибка обновления", "error", err, "message_id", msg.ID)
		return err
	}

	return nil
}

func (r *MessageRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.table())
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error("MessageRepository.Delete: ошибка удаления", "error", err, "message_id", id)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("MessageRepository.Delete: сообщение удалено", "message_id", id, "rows", rows)
	return nil
}

func (r *MessageRepository) IsAuthor(ctx context.Context, messageID, userID uuid.UUID) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1 AND user_id = $2)", r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, messageID, userID).Scan(&exists)
	if err != nil {
		r.log.Error("MessageRepository.IsAuthor: ошибка проверки", "error", err)
		return false, err
	}
	return exists, nil
}
