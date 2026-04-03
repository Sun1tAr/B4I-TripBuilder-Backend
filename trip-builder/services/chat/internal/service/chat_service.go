package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/client/event"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/redis"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

var (
	ErrMessageNotFound    = errors.New("message not found")
	ErrNotAuthorized      = errors.New("not authorized")
	ErrNotParticipant     = errors.New("user is not a participant of this event")
	ErrInvalidMessageType = errors.New("invalid message type")
)

type ChatService struct {
	msgRepo     repository.MessageRepository
	receiptRepo repository.ReadReceiptRepository
	eventClient *event.Client
	redisPubSub *redis.PubSub
	log         *logger.Logger
}

func NewChatService(
	msgRepo repository.MessageRepository,
	receiptRepo repository.ReadReceiptRepository,
	eventClient *event.Client,
	redisPubSub *redis.PubSub,
	log *logger.Logger,
) *ChatService {
	return &ChatService{
		msgRepo:     msgRepo,
		receiptRepo: receiptRepo,
		eventClient: eventClient,
		redisPubSub: redisPubSub,
		log:         log,
	}
}

// IsEventParticipant проверяет, является ли пользователь участником события
func (s *ChatService) IsEventParticipant(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	return s.eventClient.IsEventParticipant(ctx, eventID, userID)
}

// SaveMessage сохраняет сообщение в БД
func (s *ChatService) SaveMessage(ctx context.Context, eventID, userID uuid.UUID, text string) (*models.Message, error) {
	s.log.Debug("SaveMessage: начало", "event_id", eventID, "user_id", userID)

	// Проверяем, что пользователь участник
	isParticipant, err := s.eventClient.IsEventParticipant(ctx, eventID, userID)
	if err != nil {
		return nil, err
	}
	if !isParticipant {
		return nil, ErrNotParticipant
	}

	msg := &models.Message{
		ID:        uuid.New(),
		EventID:   eventID,
		UserID:    userID,
		Text:      text,
		Type:      models.MessageTypeText,
		CreatedAt: time.Now(),
	}

	if err := s.msgRepo.Create(ctx, msg); err != nil {
		s.log.Error("SaveMessage: ошибка сохранения", "error", err)
		return nil, err
	}

	s.log.Info("SaveMessage: сообщение сохранено", "message_id", msg.ID, "event_id", eventID)
	return msg, nil
}

// SaveSystemMessage сохраняет системное сообщение
func (s *ChatService) SaveSystemMessage(ctx context.Context, eventID uuid.UUID, msgType models.MessageType, text string, metadata map[string]interface{}) (*models.Message, error) {
	s.log.Debug("SaveSystemMessage: начало", "event_id", eventID, "type", msgType)

	msg := &models.Message{
		ID:        uuid.New(),
		EventID:   eventID,
		UserID:    uuid.Nil, // системное сообщение не имеет автора
		Text:      text,
		Type:      msgType,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}

	if err := s.msgRepo.Create(ctx, msg); err != nil {
		s.log.Error("SaveSystemMessage: ошибка сохранения", "error", err)
		return nil, err
	}

	s.log.Info("SaveSystemMessage: системное сообщение сохранено", "message_id", msg.ID, "event_id", eventID)
	return msg, nil
}

// GetMessages возвращает историю сообщений
func (s *ChatService) GetMessages(ctx context.Context, eventID uuid.UUID, page, size int, userID uuid.UUID) ([]models.Message, int64, error) {
	s.log.Debug("GetMessages: начало", "event_id", eventID, "page", page, "size", size)

	// Проверяем, что пользователь участник
	isParticipant, err := s.eventClient.IsEventParticipant(ctx, eventID, userID)
	if err != nil {
		return nil, 0, err
	}
	if !isParticipant {
		return nil, 0, ErrNotParticipant
	}

	offset := (page - 1) * size
	messages, total, err := s.msgRepo.GetByEventID(ctx, eventID, size, offset)
	if err != nil {
		s.log.Error("GetMessages: ошибка получения", "error", err)
		return nil, 0, err
	}

	// Отмечаем сообщения как прочитанные
	go s.MarkMessagesAsRead(context.Background(), eventID, userID)

	s.log.Info("GetMessages: сообщения получены", "event_id", eventID, "count", len(messages))
	return messages, total, nil
}

// GetRecentMessages возвращает последние N сообщений для WebSocket
func (s *ChatService) GetRecentMessages(ctx context.Context, eventID uuid.UUID, limit int) ([]models.Message, error) {
	messages, _, err := s.msgRepo.GetByEventID(ctx, eventID, limit, 0)
	return messages, err
}

// UpdateMessage обновляет сообщение
func (s *ChatService) UpdateMessage(ctx context.Context, messageID, userID uuid.UUID, newText string) (*models.Message, error) {
	s.log.Debug("UpdateMessage: начало", "message_id", messageID, "user_id", userID)

	msg, err := s.msgRepo.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, ErrMessageNotFound
	}

	// Проверяем, что пользователь автор сообщения
	isAuthor, err := s.msgRepo.IsAuthor(ctx, messageID, userID)
	if err != nil {
		return nil, err
	}
	if !isAuthor {
		return nil, ErrNotAuthorized
	}

	msg.Text = newText
	if err := s.msgRepo.Update(ctx, msg); err != nil {
		s.log.Error("UpdateMessage: ошибка обновления", "error", err)
		return nil, err
	}

	s.log.Info("UpdateMessage: сообщение обновлено", "message_id", messageID)
	return msg, nil
}

// DeleteMessage удаляет сообщение
func (s *ChatService) DeleteMessage(ctx context.Context, messageID, userID uuid.UUID) error {
	s.log.Debug("DeleteMessage: начало", "message_id", messageID, "user_id", userID)

	msg, err := s.msgRepo.GetByID(ctx, messageID)
	if err != nil {
		return err
	}
	if msg == nil {
		return ErrMessageNotFound
	}

	// Проверяем права: автор или владелец события
	isAuthor, err := s.msgRepo.IsAuthor(ctx, messageID, userID)
	if err != nil {
		return err
	}
	if isAuthor {
		return s.msgRepo.Delete(ctx, messageID)
	}

	// Проверяем, не владелец ли события
	isOwner, err := s.eventClient.IsEventOwner(ctx, msg.EventID, userID)
	if err != nil {
		return err
	}
	if !isOwner {
		return ErrNotAuthorized
	}

	if err := s.msgRepo.Delete(ctx, messageID); err != nil {
		s.log.Error("DeleteMessage: ошибка удаления", "error", err)
		return err
	}

	s.log.Info("DeleteMessage: сообщение удалено", "message_id", messageID)
	return nil
}

// MarkMessagesAsRead отмечает все сообщения в событии как прочитанные
func (s *ChatService) MarkMessagesAsRead(ctx context.Context, eventID, userID uuid.UUID) error {
	s.log.Debug("MarkMessagesAsRead: начало", "event_id", eventID, "user_id", userID)
	return s.receiptRepo.MarkAsRead(ctx, eventID, userID)
}

// GetUnreadCount возвращает количество непрочитанных сообщений
func (s *ChatService) GetUnreadCount(ctx context.Context, eventID, userID uuid.UUID) (int64, error) {
	s.log.Debug("GetUnreadCount: начало", "event_id", eventID, "user_id", userID)
	return s.receiptRepo.GetUnreadCount(ctx, eventID, userID)
}

// PublishMessage публикует сообщение в Redis для рассылки
func (s *ChatService) PublishMessage(ctx context.Context, eventID uuid.UUID, msg *models.Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		s.log.Error("PublishMessage: ошибка маршалинга", "error", err)
		return
	}

	if err := s.redisPubSub.Publish(ctx, eventID, data); err != nil {
		s.log.Error("PublishMessage: ошибка публикации", "error", err)
	}
}
