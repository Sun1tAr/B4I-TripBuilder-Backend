package repository

import (
	"context"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/domain/models"
	"github.com/google/uuid"
)

type MessageRepository interface {
	Create(ctx context.Context, msg *models.Message) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Message, error)
	GetByEventID(ctx context.Context, eventID uuid.UUID, limit, offset int) ([]models.Message, int64, error)
	Update(ctx context.Context, msg *models.Message) error
	Delete(ctx context.Context, id uuid.UUID) error
	IsAuthor(ctx context.Context, messageID, userID uuid.UUID) (bool, error)
}

type ReadReceiptRepository interface {
	Create(ctx context.Context, receipt *models.ReadReceipt) error
	GetUnreadCount(ctx context.Context, eventID, userID uuid.UUID) (int64, error)
	MarkAsRead(ctx context.Context, eventID, userID uuid.UUID) error
}
