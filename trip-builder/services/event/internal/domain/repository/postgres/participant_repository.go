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

func (r *ParticipantRepository) eventsTable() string {
	return fmt.Sprintf("%s.events", r.schema)
}

// Add добавляет участника или приглашение
func (r *ParticipantRepository) Add(ctx context.Context, participant *models.Participant) error {
	if participant.InviteID == uuid.Nil {
		participant.InviteID = uuid.New()
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (invite_id, event_id, user_id, role, approved_by_owner, approved_by_user, invited_email, joined_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`, r.table())

	_, err := r.db.ExecContext(ctx, query,
		participant.InviteID, participant.EventID, participant.UserID,
		participant.Role, participant.ApprovedByOwner, participant.ApprovedByUser,
		participant.InvitedEmail,
	)
	if err != nil {
		r.log.Error("ParticipantRepository.Add: ошибка добавления", "error", err, "event_id", participant.EventID)
		return err
	}

	return nil
}

// AddBatch добавляет нескольких участников массово
func (r *ParticipantRepository) AddBatch(ctx context.Context, eventID uuid.UUID, userIDs []uuid.UUID) error {
	if len(userIDs) == 0 {
		return nil
	}

	valueStrings := make([]string, 0, len(userIDs))
	args := make([]interface{}, 0, len(userIDs)*5)
	argPos := 1

	for _, userID := range userIDs {
		inviteID := uuid.New()
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, 'member', true, false, NULL, NOW())", argPos, argPos+1, argPos+2))
		args = append(args, inviteID, eventID, userID)
		argPos += 3
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (invite_id, event_id, user_id, role, approved_by_owner, approved_by_user, invited_email, joined_at)
		VALUES %s
	`, r.table(), strings.Join(valueStrings, ","))

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.log.Error("ParticipantRepository.AddBatch: ошибка массового добавления", "error", err, "event_id", eventID)
		return err
	}

	return nil
}

// GetByEventID получает всех участников события
func (r *ParticipantRepository) GetByEventID(ctx context.Context, eventID uuid.UUID) ([]models.Participant, error) {
	query := fmt.Sprintf(`
		SELECT invite_id, event_id, user_id, role, approved_by_owner, approved_by_user, invited_email, joined_at
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

		err := rows.Scan(
			&p.InviteID,
			&p.EventID,
			&p.UserID,
			&p.Role,
			&p.ApprovedByOwner,
			&p.ApprovedByUser,
			&invitedEmail,
			&p.JoinedAt,
		)
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

// GetByUserAndEvent получает участника по событию и пользователю
func (r *ParticipantRepository) GetByUserAndEvent(ctx context.Context, eventID, userID uuid.UUID) (*models.Participant, error) {
	query := fmt.Sprintf(`
		SELECT invite_id, event_id, user_id, role, approved_by_owner, approved_by_user, invited_email, joined_at
		FROM %s WHERE event_id = $1 AND user_id = $2
	`, r.table())

	p := &models.Participant{}
	var invitedEmail sql.NullString

	err := r.db.QueryRowContext(ctx, query, eventID, userID).Scan(
		&p.InviteID, &p.EventID, &p.UserID, &p.Role,
		&p.ApprovedByOwner, &p.ApprovedByUser, &invitedEmail, &p.JoinedAt,
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

// GetByInviteID получает участника по invite_id
func (r *ParticipantRepository) GetByInviteID(ctx context.Context, inviteID uuid.UUID) (*models.Participant, error) {
	query := fmt.Sprintf(`
		SELECT invite_id, event_id, user_id, role, approved_by_owner, approved_by_user, invited_email, joined_at
		FROM %s WHERE invite_id = $1
	`, r.table())

	p := &models.Participant{}
	var invitedEmail sql.NullString

	err := r.db.QueryRowContext(ctx, query, inviteID).Scan(
		&p.InviteID, &p.EventID, &p.UserID, &p.Role,
		&p.ApprovedByOwner, &p.ApprovedByUser, &invitedEmail, &p.JoinedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error("ParticipantRepository.GetByInviteID: ошибка", "error", err, "invite_id", inviteID)
		return nil, err
	}

	if invitedEmail.Valid {
		p.InvitedEmail = invitedEmail.String
	}

	return p, nil
}

// GetInvitesByEmail возвращает приглашения пользователя по email (где approved_by_owner = true, approved_by_user = false)
func (r *ParticipantRepository) GetInvitesByEmail(ctx context.Context, email string) ([]models.Invite, error) {
	query := fmt.Sprintf(`
		SELECT 
			p.invite_id,
			e.id as event_id,
			e.title as event_title,
			e.description as event_description,
			e.start_date as event_start_date,
			e.end_date as event_end_date,
			e.owner_id as event_owner_id,
			COALESCE((
				SELECT COUNT(*) 
				FROM %s p2 
				WHERE p2.event_id = e.id 
				  AND p2.approved_by_owner = true 
				  AND p2.approved_by_user = true
			), 0) as participants_count,
			p.user_id as invited_by,
			p.joined_at as invited_at
		FROM %s p
		JOIN %s e ON p.event_id = e.id
		WHERE p.invited_email = $1 
		  AND p.approved_by_owner = true 
		  AND p.approved_by_user = false
		ORDER BY p.joined_at DESC
	`, r.table(), r.table(), r.eventsTable())

	rows, err := r.db.QueryContext(ctx, query, email)
	if err != nil {
		r.log.Error("ParticipantRepository.GetInvitesByEmail: ошибка запроса", "error", err, "email", email)
		return nil, err
	}
	defer rows.Close()

	var invites []models.Invite
	for rows.Next() {
		var invite models.Invite
		var description sql.NullString
		var invitedBy sql.NullString

		err := rows.Scan(
			&invite.InviteID,
			&invite.EventID,
			&invite.EventTitle,
			&description,
			&invite.EventStartDate,
			&invite.EventEndDate,
			&invite.EventOwnerID,
			&invite.ParticipantsCount,
			&invitedBy,
			&invite.InvitedAt,
		)
		if err != nil {
			r.log.Error("ParticipantRepository.GetInvitesByEmail: ошибка сканирования", "error", err)
			return nil, err
		}

		if description.Valid {
			invite.EventDescription = description.String
		}
		if invitedBy.Valid {
			invite.InvitedBy, _ = uuid.Parse(invitedBy.String)
		}

		invites = append(invites, invite)
	}

	r.log.Debug("ParticipantRepository.GetInvitesByEmail: получены приглашения", "email", email, "count", len(invites))
	return invites, nil
}

// UpdateApproval обновляет флаги подтверждения по invite_id
func (r *ParticipantRepository) UpdateApproval(ctx context.Context, inviteID uuid.UUID, approveByOwner, approveByUser bool) error {
	query := fmt.Sprintf(`
		UPDATE %s SET approved_by_owner = $1, approved_by_user = $2
		WHERE invite_id = $3
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, approveByOwner, approveByUser, inviteID)
	if err != nil {
		r.log.Error("ParticipantRepository.UpdateApproval: ошибка обновления",
			"error", err, "invite_id", inviteID)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("ParticipantRepository.UpdateApproval: обновлено",
		"invite_id", inviteID, "rows", rows)
	return nil
}

// UpdateByInviteID обновляет user_id и флаги подтверждения по invite_id
func (r *ParticipantRepository) UpdateByInviteID(ctx context.Context, inviteID, userID uuid.UUID, approveByOwner, approveByUser bool) error {
	query := fmt.Sprintf(`
		UPDATE %s SET user_id = $1, approved_by_owner = $2, approved_by_user = $3, invited_email = NULL
		WHERE invite_id = $4
	`, r.table())

	result, err := r.db.ExecContext(ctx, query, userID, approveByOwner, approveByUser, inviteID)
	if err != nil {
		r.log.Error("ParticipantRepository.UpdateByInviteID: ошибка обновления",
			"error", err, "invite_id", inviteID, "user_id", userID)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("ParticipantRepository.UpdateByInviteID: обновлено",
		"invite_id", inviteID, "user_id", userID, "rows", rows)
	return nil
}

// Remove удаляет участника по event_id и user_id
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

// RemoveByInviteID удаляет участника по invite_id
func (r *ParticipantRepository) RemoveByInviteID(ctx context.Context, inviteID uuid.UUID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE invite_id = $1", r.table())
	result, err := r.db.ExecContext(ctx, query, inviteID)
	if err != nil {
		r.log.Error("ParticipantRepository.RemoveByInviteID: ошибка удаления", "error", err, "invite_id", inviteID)
		return err
	}

	rows, _ := result.RowsAffected()
	r.log.Debug("ParticipantRepository.RemoveByInviteID: приглашение удалено", "invite_id", inviteID, "rows", rows)
	return nil
}

// IsParticipant проверяет, является ли пользователь полноценным участником (оба флага true)
func (r *ParticipantRepository) IsParticipant(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	query := fmt.Sprintf(`
		SELECT EXISTS(SELECT 1 FROM %s 
		WHERE event_id = $1 AND user_id = $2 
		  AND approved_by_owner = true AND approved_by_user = true)
	`, r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, eventID, userID).Scan(&exists)
	if err != nil {
		r.log.Error("ParticipantRepository.IsParticipant: ошибка", "error", err, "event_id", eventID, "user_id", userID)
		return false, err
	}
	return exists, nil
}

// IsOwner проверяет, является ли пользователь владельцем (роль owner и approved_by_owner = true)
func (r *ParticipantRepository) IsOwner(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	query := fmt.Sprintf(`
		SELECT EXISTS(SELECT 1 FROM %s 
		WHERE event_id = $1 AND user_id = $2 
		  AND role = 'owner' AND approved_by_owner = true)
	`, r.table())
	var exists bool
	err := r.db.QueryRowContext(ctx, query, eventID, userID).Scan(&exists)
	if err != nil {
		r.log.Error("ParticipantRepository.IsOwner: ошибка", "error", err, "event_id", eventID, "user_id", userID)
		return false, err
	}
	return exists, nil
}

// CountByEventID считает количество полностью подтверждённых участников (оба флага true)
func (r *ParticipantRepository) CountByEventID(ctx context.Context, eventID uuid.UUID) (int, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s 
		WHERE event_id = $1 
		  AND approved_by_owner = true 
		  AND approved_by_user = true
	`, r.table())
	var count int
	err := r.db.QueryRowContext(ctx, query, eventID).Scan(&count)
	if err != nil {
		r.log.Error("ParticipantRepository.CountByEventID: ошибка подсчёта", "error", err, "event_id", eventID)
		return 0, err
	}
	return count, nil
}

// FindByInvitedEmail находит приглашение по email
func (r *ParticipantRepository) FindByInvitedEmail(ctx context.Context, eventID uuid.UUID, email string) (*models.Participant, error) {
	query := fmt.Sprintf(`
		SELECT invite_id, event_id, user_id, role, approved_by_owner, approved_by_user, invited_email, joined_at
		FROM %s WHERE event_id = $1 AND invited_email = $2
	`, r.table())

	p := &models.Participant{}
	var invitedEmail sql.NullString
	var userID sql.NullString

	err := r.db.QueryRowContext(ctx, query, eventID, email).Scan(
		&p.InviteID, &p.EventID, &userID, &p.Role,
		&p.ApprovedByOwner, &p.ApprovedByUser, &invitedEmail, &p.JoinedAt,
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

// LinkInvitedParticipant связывает приглашение с пользователем по invite_id
func (r *ParticipantRepository) LinkInvitedParticipant(ctx context.Context, eventID, userID uuid.UUID, email string) error {
	// Ищем приглашение по event_id и invited_email
	participant, err := r.FindByInvitedEmail(ctx, eventID, email)
	if err != nil {
		return err
	}
	if participant == nil {
		// Нет приглашения, создаём нового участника
		newParticipant := &models.Participant{
			EventID:         eventID,
			UserID:          userID,
			Role:            "member",
			ApprovedByOwner: false,
			ApprovedByUser:  true, // пользователь сам хочет присоединиться
		}
		return r.Add(ctx, newParticipant)
	}

	// Обновляем существующее приглашение
	return r.UpdateByInviteID(ctx, participant.InviteID, userID, participant.ApprovedByOwner, true)
}

// LinkAllInvitationsByEmail связывает все приглашения с указанным email с пользователем
func (r *ParticipantRepository) LinkAllInvitationsByEmail(ctx context.Context, userID uuid.UUID, email string) error {
	// Находим все приглашения с этим email, где approved_by_owner = true, approved_by_user = false
	query := fmt.Sprintf(`
		UPDATE %s SET user_id = $1, approved_by_user = true, invited_email = NULL
		WHERE invited_email = $2 
		  AND approved_by_owner = true 
		  AND approved_by_user = false
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
