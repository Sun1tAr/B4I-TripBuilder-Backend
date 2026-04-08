package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/user/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/segmentio/kafka-go"
)

type RouteCreatedConsumer struct {
	reader       *kafka.Reader
	statsService *service.StatsService
	log          *logger.Logger
}

func NewRouteCreatedConsumer(brokers []string, topic string, groupID string, statsService *service.StatsService, log *logger.Logger) *RouteCreatedConsumer {
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
	return &RouteCreatedConsumer{
		reader:       reader,
		statsService: statsService,
		log:          log,
	}
}

func (c *RouteCreatedConsumer) Start(ctx context.Context) {
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

func (c *RouteCreatedConsumer) processMessage(ctx context.Context, msg kafka.Message) {

	var base BaseEvent
	if err := json.Unmarshal(msg.Value, &base); err != nil {
		c.log.Error("Ошибка парсинга базового события", "error", err)
		return
	}

	var data RouteCreatedData
	if err := json.Unmarshal(base.Data, &data); err != nil {
		c.log.Error("Ошибка парсинга данных route.created", "error", err)
		return
	}

	c.log.Debug("Обработка route.created", "user_id", data.UserID, "route_id", data.RouteID)

	if err := c.statsService.IncrementRoutesCreated(ctx, data.UserID); err != nil {
		c.log.Error("Ошибка инкремента routes_created", "error", err, "user_id", data.UserID)
		return
	}

	c.log.Info("routes_created увеличен", "user_id", data.UserID)
}

func (c *RouteCreatedConsumer) Close() error {
	return c.reader.Close()
}
