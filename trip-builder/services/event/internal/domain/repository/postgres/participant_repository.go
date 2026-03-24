package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

type ParticipantRepository struct {
	db     *sql.DB
	schema string
	log    *logger.Logger
}

func NewParticipantRepository(db *sql.DB, schema string, log *logger.Logger) *ParticipantRepository {
	return &ParticipantRepository{
		db:     db,
		schema: schema,
		log:    log,
	}
}

func (r *ParticipantRepository) table() string {
	return fmt.Sprintf("%s.event_participants", r.schema)
}

func (r *ParticipantRepository) Add(ctx context.Context, participant *models.Participant) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (event_id, user_id, role, status, invited_email, joined_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (event_id, user_id) DO UPDATE SET
			status = EXCLUDED.status,
			role = EXCLUDED.role
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		participant.EventID, participant.UserID, participant.Role,
		participant.Status, participant.InvitedEmail,
	)
	if err != nil {
		r.log.Error("ParticipantRepository.Add: ошибка добавления", "error", err, "event_id", participant.EventID)
		return err
	}

	return nil
}

func (r *ParticipantRepository) AddBatch(ctx context.Context, eventID uuid.UUID, userIDs []uuid.UUID) error {
	if len(userIDs) == 0 {
		return nil
	}

	valueStrings := make([]string, 0, len(userIDs))
	args := make([]interface{}, 0, len(userIDs)*3)
	argPos := 1

	for _, userID := range userIDs {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, 'member', 'accepted', NULL, NOW())", argPos, argPos+1))
		args = append(args, eventID, userID)
		argPos += 2
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (event_id, user_id, role, status, invited_email, joined_at)
		VALUES %s
		ON CONFLICT (event_id, user_id) DO NOTHING
	`, r.table(), strings.Join(valueStrings, ","))

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.log.Error("ParticipantRepository.AddBatch: ошибка массового добавления", "error", err, "event_id", eventID)
		return err
	}

	return nil
}

func (r *ParticipantRepository) GetByEventID(ctx context.Context, eventID uuid.UUID) ([]models.Participant, error) {
	query := fmt.Sprintf(`
		SELECT event_id, user_id, role, status, invited_email, joined_at
		FROM %s WHERE event_id = $1
	`, r.table())

	rows, err := r.db.QueryContext(ctx, query, eventID)
	if err != nil {
		r.log.Error("ParticipantRepository.GetByEventID: ошибка запроса", "error", err, "event_id", eventID)
		return nil, err
	}
	defer rows.Close()

	var participants []models.Participant
	for rows.Next() {
		var p models.Participant
		var invitedEmail sql.NullString

		err := rows.Scan(&p.EventID, &p.UserID, &p.Role, &p.Status, &invitedEmail, &p.JoinedAt)
		if err != nil {
			r.log.Error("ParticipantRepository.GetByEventID: ошибка сканирования", "error", err)
			return nil, err
		}

		if invitedEmail.Valid {
			p.InvitedEmail = invitedEmail.String
		}
		participants = append(participants, p)
	}

	return participants, nil
}

func (r *ParticipantRepository) GetByUserAndEvent(ctx context.Context, eventID, userID uuid.UUID) (*models.Participant, error) {
	query := fmt.Sprintf(`
		SELECT event_id, user_id, role, status, invited_email, joined_at
		FROM %s WHERE event_id = $1 AND user_id = $2
	`, r.table())

	p := &models.Participant{}
	var invitedEmail sql.NullString

	err := r.db.QueryRowContext(ctx, query, eventID, userID).Scan(
		&p.EventID, &p.UserID, &p.Role, &p.Status, &invitedEmail, &p.JoinedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("ParticipantRepository.GetByUserAndEvent: ошибка", "error", err, "event_id", eventID, "user_id", userID)
		return nil, err
	}

	if invitedEmail.Valid {
		p.InvitedEmail = invitedEmail.String
	}

	return p, nil
}

func (r *ParticipantRepository) UpdateStatus(ctx context.Context, eventID, userID uuid.UUID, status string) error {
	query := fmt.Sprintf("UPDATE %s SET status = $1 WHERE event_id = $2 AND user_id = $3", r.table())
	_, err := r.db.ExecContext(ctx, query, status, eventID, userID)
	if err != nil {
		r.log.Error("ParticipantRepository.UpdateStatus: ошибка", "error", err, "event_id", eventID, "user_id", userID)
		return err
	}
	return nil
}

func (r *ParticipantRepository) Remove(ctx context.Context, eventID, userID uuid.UUID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE event_id = $1 AND user_id = $2", r.table())
	result, err := r.db.ExecContext(ctx, query, eventID, userID)
	if err != nil {
		r.log.Error("ParticipantRepository.Remove: ошибка удаления", "error", err, "event_id", eventID, "user_id", userID)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("ParticipantRepository.Remove: участник удалён", "event_id", eventID, "user_id", userID, "rows", rows)
	return nil
}

// IsParticipant проверяет, является ли пользователь участником со статусом accepted
func (r *ParticipantRepository) IsParticipant(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	query := fmt.Sprintf(`
		SELECT EXISTS(SELECT 1 FROM %s 
		WHERE event_id = $1 AND user_id = $2 AND status = 'accepted')
	`, r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, eventID, userID).Scan(&exists)
	if err != nil {
		r.log.Error("ParticipantRepository.IsParticipant: ошибка", "error", err, "event_id", eventID, "user_id", userID)
		return false, err
	}
	return exists, nil
}

// IsOwner проверяет, является ли пользователь владельцем со статусом accepted
func (r *ParticipantRepository) IsOwner(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	query := fmt.Sprintf(`
		SELECT EXISTS(SELECT 1 FROM %s 
		WHERE event_id = $1 AND user_id = $2 AND role = 'owner' AND status = 'accepted')
	`, r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, eventID, userID).Scan(&exists)
	if err != nil {
		r.log.Error("ParticipantRepository.IsOwner: ошибка", "error", err, "event_id", eventID, "user_id", userID)
		return false, err
	}
	return exists, nil
}

func (r *ParticipantRepository) CountByEventID(ctx context.Context, eventID uuid.UUID) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE event_id = $1 AND status = 'accepted'", r.table())
	var count int
	err := r.db.QueryRowContext(ctx, query, eventID).Scan(&count)
	if err != nil {
		r.log.Error("ParticipantRepository.CountByEventID: ошибка подсчёта", "error", err, "event_id", eventID)
		return 0, err
	}
	return count, nil
}

func (r *ParticipantRepository) FindByInvitedEmail(ctx context.Context, eventID uuid.UUID, email string) (*models.Participant, error) {
	query := fmt.Sprintf(`
		SELECT event_id, user_id, role, status, invited_email, joined_at
		FROM %s WHERE event_id = $1 AND invited_email = $2
	`, r.table())

	p := &models.Participant{}
	var invitedEmail sql.NullString
	var userID sql.NullString

	err := r.db.QueryRowContext(ctx, query, eventID, email).Scan(
		&p.EventID, &userID, &p.Role, &p.Status, &invitedEmail, &p.JoinedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("ParticipantRepository.FindByInvitedEmail: ошибка", "error", err, "event_id", eventID, "email", email)
		return nil, err
	}

	if userID.Valid {
		p.UserID, _ = uuid.Parse(userID.String)
	}
	if invitedEmail.Valid {
		p.InvitedEmail = invitedEmail.String
	}

	return p, nil
}

func (r *ParticipantRepository) LinkInvitedParticipant(ctx context.Context, eventID, userID uuid.UUID, email string) error {
	query := fmt.Sprintf(`
		UPDATE %s SET user_id = $1, invited_email = NULL
		WHERE event_id = $2 AND invited_email = $3
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, userID, eventID, email)
	if err != nil {
		r.log.Error("ParticipantRepository.LinkInvitedParticipant: ошибка", "error", err, "event_id", eventID, "user_id", userID)
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Если не нашли по invited_email, добавляем как нового участника
		participant := &models.Participant{
			EventID: eventID,
			UserID:  userID,
			Role:    "member",
			Status:  "accepted",
		}
		return r.Add(ctx, participant)
	}

	return nil
}

// LinkAllInvitationsByEmail связывает все приглашения с указанным email с пользователем
func (r *ParticipantRepository) LinkAllInvitationsByEmail(ctx context.Context, userID uuid.UUID, email string) error {
	query := fmt.Sprintf(`
		UPDATE %s SET user_id = $1, invited_email = NULL
		WHERE invited_email = $2 AND status = 'pending'
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, userID, email)
	if err != nil {
		r.log.Error("ParticipantRepository.LinkAllInvitationsByEmail: ошибка обновления",
			"error", err, "user_id", userID, "email", email)
		return err
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		r.log.Info("ParticipantRepository.LinkAllInvitationsByEmail: приглашения связаны",
			"user_id", userID, "email", email, "count", rows)
	} else {
		r.log.Debug("ParticipantRepository.LinkAllInvitationsByEmail: приглашения не найдены",
			"user_id", userID, "email", email)
	}

	return nil
}
