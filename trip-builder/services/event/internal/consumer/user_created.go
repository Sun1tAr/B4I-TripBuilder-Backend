package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/event/internal/domain/repository"
	authdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/auth"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/segmentio/kafka-go"
)

type UserCreatedConsumer struct {
	reader          *kafka.Reader
	participantRepo repository.ParticipantRepository
	log             *logger.Logger
}

func NewUserCreatedConsumer(
	brokers []string,
	topic string,
	groupID string,
	participantRepo repository.ParticipantRepository,
	log *logger.Logger,
) *UserCreatedConsumer {
	log.Debug("Создание UserCreatedConsumer",
		"brokers", brokers,
		"topic", topic,
		"group_id", groupID,
	)
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:           brokers,
		Topic:             topic,
		GroupID:           groupID,
		MinBytes:          1, // НЕ ждём накопления
		MaxBytes:          10e6,
		MaxWait:           500 * time.Millisecond, // Чаще отправляем heartbeat
		CommitInterval:    5 * time.Second,        // Асинхронный коммит (не блокирует)
		SessionTimeout:    30 * time.Second,       // Время жизни сессии
		HeartbeatInterval: 3 * time.Second,        // Частота heartbeat
	})

	return &UserCreatedConsumer{
		reader:          reader,
		participantRepo: participantRepo,
		log:             log,
	}
}

func (c *UserCreatedConsumer) Start(ctx context.Context) {
	c.log.Info("Запуск consumer для топика", "topic", c.reader.Config().Topic)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Остановка consumer", "topic", c.reader.Config().Topic)
			return
		default:
			msg, err := c.reader.FetchMessage(ctx) // Используем FetchMessage вместо ReadMessage
			if err != nil {
				// Если reader закрыт, выходим
				if errors.Is(err, context.Canceled) {
					return
				}
				c.log.Error("Ошибка чтения сообщения", "error", err)
				time.Sleep(time.Second) // Не спамим ошибками
				continue
			}

			c.processMessage(ctx, msg)

			// Ручной коммит
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.log.Error("Ошибка коммита", "error", err)
			}
		}
	}
}

func (c *UserCreatedConsumer) processMessage(ctx context.Context, msg kafka.Message) {

	// Парсим событие
	var baseEvent struct {
		EventID   string          `json:"event_id"`
		EventType string          `json:"event_type"`
		Timestamp time.Time       `json:"timestamp"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(msg.Value, &baseEvent); err != nil {
		c.log.Error("Ошибка парсинга базового события", "error", err)
		return
	}

	// Проверяем тип события
	if baseEvent.EventType != "user.created" {
		c.log.Debug("Пропуск события другого типа", "event_type", baseEvent.EventType)
		return
	}

	// Парсим данные
	var data authdto.UserCreatedData
	if err := json.Unmarshal(baseEvent.Data, &data); err != nil {
		c.log.Error("Ошибка парсинга данных user.created", "error", err)
		return
	}

	c.log.Debug("Обработка user.created",
		"user_id", data.UserID,
		"email", data.Email,
	)

	// Находим все приглашения с этим email
	invites, err := c.participantRepo.FindByInvitedEmail(ctx, data.Email)
	if err != nil {
		c.log.Error("Ошибка поиска приглашений по email", "error", err, "email", data.Email)
		return
	}

	// Связываем каждое приглашение с пользователем
	for _, invite := range invites {
		if err := c.participantRepo.LinkInvitation(ctx, invite.InviteID, data.UserID); err != nil {
			c.log.Error("Ошибка связывания приглашения", "error", err, "invite_id", invite.InviteID, "user_id", data.UserID)
		} else {
			c.log.Info("Приглашение связано с пользователем", "invite_id", invite.InviteID, "user_id", data.UserID, "email", data.Email)
		}
	}
}
