package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type ReadReceiptRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewReadReceiptRepository(db *sql.DB, schema string, log *logger.Logger) *ReadReceiptRepository {
	return &ReadReceiptRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *ReadReceiptRepository) table() string {
	return fmt.Sprintf("%s.read_receipts", r.schema)
}

func (r *ReadReceiptRepository) messagesTable() string {
	return fmt.Sprintf("%s.messages", r.schema)
}

func (r *ReadReceiptRepository) Create(ctx context.Context, receipt *models.ReadReceipt) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (message_id, user_id, read_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (message_id, user_id) DO NOTHING
	`, r.table())

	_, err := r.db.ExecContext(ctx, query, receipt.MessageID, receipt.UserID, receipt.ReadAt)
	if err != nil {
		r.log.Error("ReadReceiptRepository.Create: ошибка создания", "error", err)
		return err
	}

	return nil
}

func (r *ReadReceiptRepository) GetUnreadCount(ctx context.Context, eventID, userID uuid.UUID) (int64, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s m
		LEFT JOIN %s rr ON m.id = rr.message_id AND rr.user_id = $2
		WHERE m.event_id = $1 AND rr.message_id IS NULL AND m.user_id != $2
	`, r.messagesTable(), r.table())

	var count int64
	err := r.db.QueryRowContext(ctx, query, eventID, userID).Scan(&count)
	if err != nil {
		r.log.Error("ReadReceiptRepository.GetUnreadCount: ошибка", "error", err)
		return 0, err
	}

	return count, nil
}

func (r *ReadReceiptRepository) MarkAsRead(ctx context.Context, eventID, userID uuid.UUID) error {
	// Вставляем записи о прочтении для всех непрочитанных сообщений
	query := fmt.Sprintf(`
		INSERT INTO %s (message_id, user_id, read_at)
		SELECT m.id, $1, NOW()
		FROM %s m
		LEFT JOIN %s rr ON m.id = rr.message_id AND rr.user_id = $1
		WHERE m.event_id = $2 AND rr.message_id IS NULL AND m.user_id != $1
	`, r.table(), r.messagesTable(), r.table())

	_, err := r.db.ExecContext(ctx, query, userID, eventID)
	if err != nil {
		r.log.Error("ReadReceiptRepository.MarkAsRead: ошибка", "error", err)
		return err
	}

	return nil
}
