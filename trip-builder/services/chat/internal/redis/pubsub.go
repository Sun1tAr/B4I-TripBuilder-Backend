package redis

import (
	"context"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type PubSub struct {
	client *redis.Client
	log    *logger.Logger
}

func NewPubSub(addr, password string, db int, log *logger.Logger) *PubSub {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &PubSub{
		client: client,
		log:    log,
	}
}

// Publish публикует сообщение в канал комнаты
func (p *PubSub) Publish(ctx context.Context, eventID uuid.UUID, data []byte) error {
	channel := fmt.Sprintf("chat:room:%s", eventID)
	return p.client.Publish(ctx, channel, data).Err()
}

// Subscribe подписывается на канал комнаты
func (p *PubSub) Subscribe(ctx context.Context, eventID uuid.UUID) (*redis.PubSub, error) {
	channel := fmt.Sprintf("chat:room:%s", eventID)
	pubsub := p.client.Subscribe(ctx, channel)

	// Ждём подтверждения подписки
	_, err := pubsub.Receive(ctx)
	if err != nil {
		return nil, err
	}

	return pubsub, nil
}

// Close закрывает соединение
func (p *PubSub) Close() error {
	return p.client.Close()
}
