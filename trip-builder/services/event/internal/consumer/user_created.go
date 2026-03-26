package consumer

import (
	"context"
	"encoding/json"
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
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3,
		MaxBytes: 10e6,
		MaxWait:  1 * time.Second,
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
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				c.log.Error("Ошибка чтения сообщения", "error", err)
				continue
			}

			c.processMessage(ctx, msg)
		}
	}
}

func (c *UserCreatedConsumer) processMessage(ctx context.Context, msg kafka.Message) {
	log := c.log.With(
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
	)

	// Парсим событие
	var baseEvent struct {
		EventID   string          `json:"event_id"`
		EventType string          `json:"event_type"`
		Timestamp time.Time       `json:"timestamp"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(msg.Value, &baseEvent); err != nil {
		log.Error("Ошибка парсинга базового события", "error", err)
		return
	}

	// Проверяем тип события
	if baseEvent.EventType != "user.created" {
		log.Debug("Пропуск события другого типа", "event_type", baseEvent.EventType)
		return
	}

	// Парсим данные
	var data authdto.UserCreatedData
	if err := json.Unmarshal(baseEvent.Data, &data); err != nil {
		log.Error("Ошибка парсинга данных user.created", "error", err)
		return
	}

	log.Debug("Обработка user.created",
		"user_id", data.UserID,
		"email", data.Email,
	)

	// Находим все приглашения с этим email
	invites, err := c.participantRepo.FindByInvitedEmail(ctx, data.Email)
	if err != nil {
		log.Error("Ошибка поиска приглашений по email", "error", err, "email", data.Email)
		return
	}

	// Связываем каждое приглашение с пользователем
	for _, invite := range invites {
		if err := c.participantRepo.LinkInvitation(ctx, invite.InviteID, data.UserID); err != nil {
			log.Error("Ошибка связывания приглашения", "error", err, "invite_id", invite.InviteID, "user_id", data.UserID)
		} else {
			log.Info("Приглашение связано с пользователем", "invite_id", invite.InviteID, "user_id", data.UserID, "email", data.Email)
		}
	}
}
