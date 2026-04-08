package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/segmentio/kafka-go"
)

type MediaUploadedConsumer struct {
	reader       *kafka.Reader
	statsService *service.StatsService
	log          *logger.Logger
}

func NewMediaUploadedConsumer(brokers []string, topic string, groupID string, statsService *service.StatsService, log *logger.Logger) *MediaUploadedConsumer {
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
	return &MediaUploadedConsumer{
		reader:       reader,
		statsService: statsService,
		log:          log,
	}
}

func (c *MediaUploadedConsumer) Start(ctx context.Context) {
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

func (c *MediaUploadedConsumer) processMessage(ctx context.Context, msg kafka.Message) {

	var base BaseEvent
	if err := json.Unmarshal(msg.Value, &base); err != nil {
		c.log.Error("Ошибка парсинга базового события", "error", err)
		return
	}

	var data MediaUploadedData
	if err := json.Unmarshal(base.Data, &data); err != nil {
		c.log.Error("Ошибка парсинга данных media.uploaded", "error", err)
		return
	}

	c.log.Debug("Обработка media.uploaded", "user_id", data.UserID, "media_id", data.MediaID)

	if err := c.statsService.IncrementPhotosUploaded(ctx, data.UserID); err != nil {
		c.log.Error("Ошибка инкремента photos_uploaded", "error", err, "user_id", data.UserID)
		return
	}

	c.log.Info("photos_uploaded увеличен", "user_id", data.UserID)
}

func (c *MediaUploadedConsumer) Close() error {
	return c.reader.Close()
}
