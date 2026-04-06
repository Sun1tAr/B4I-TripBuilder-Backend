package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	client   *minio.Client
	bucket   string
	endpoint string
	useSSL   bool
	log      *logger.Logger
}

type UploadResult struct {
	Key  string
	Size int64
}

func NewClient(endpoint, accessKey, secretKey, bucket string, useSSL bool, log *logger.Logger) (*Client, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	// Проверяем существование бакета, создаём если нет
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		log.Info("Бакет создан", "bucket", bucket)
	}

	return &Client{
		client:   client,
		bucket:   bucket,
		endpoint: endpoint,
		useSSL:   useSSL,
		log:      log,
	}, nil
}

// GenerateObjectKey генерирует уникальный ключ для объекта
func (c *Client) GenerateObjectKey(purpose string, userID uuid.UUID, filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	// Определяем папку в зависимости от назначения
	var folder string
	switch purpose {
	case "avatar":
		folder = "avatars"
	case "route_image":
		folder = "routes"
	case "event_photo":
		folder = "events"
	case "waypoint_photo":
		folder = "waypoints"
	default:
		folder = "general"
	}

	objectKey := fmt.Sprintf("%s/%s/%s%s", folder, userID.String(), uuid.New().String(), ext)
	return objectKey
}

// GenerateObjectKeyForEvent генерирует ключ для медиа поездки
func (c *Client) GenerateObjectKeyForEvent(eventID, userID uuid.UUID, filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	objectKey := fmt.Sprintf("events/%s/%s%s", eventID.String(), uuid.New().String(), ext)
	return objectKey
}

// GenerateObjectKeyForWaypoint генерирует ключ для медиа точки
func (c *Client) GenerateObjectKeyForWaypoint(eventID, waypointID, userID uuid.UUID, filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	objectKey := fmt.Sprintf("events/%s/waypoints/%s/%s%s", eventID.String(), waypointID.String(), uuid.New().String(), ext)
	return objectKey
}

// Upload загружает файл в MinIO
func (c *Client) Upload(ctx context.Context, objectKey string, data []byte, contentType string) (*UploadResult, error) {
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(objectKey))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	reader := bytes.NewReader(data)
	info, err := c.client.PutObject(ctx, c.bucket, objectKey, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		c.log.Error("MinIO.Upload: ошибка загрузки", "error", err, "key", objectKey)
		return nil, fmt.Errorf("failed to upload object: %w", err)
	}

	c.log.Debug("MinIO.Upload: файл загружен", "key", objectKey, "size", info.Size)
	return &UploadResult{
		Key:  objectKey,
		Size: info.Size,
	}, nil
}

// Download скачивает файл из MinIO
func (c *Client) Download(ctx context.Context, objectKey string) ([]byte, string, error) {
	reader, err := c.client.GetObject(ctx, c.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object: %w", err)
	}
	defer reader.Close()

	// Получаем информацию об объекте для Content-Type
	stat, err := reader.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object stat: %w", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read object: %w", err)
	}

	contentType := stat.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return data, contentType, nil
}

// Delete удаляет файл из MinIO
func (c *Client) Delete(ctx context.Context, objectKey string) error {
	err := c.client.RemoveObject(ctx, c.bucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		c.log.Error("MinIO.Delete: ошибка удаления", "error", err, "key", objectKey)
		return fmt.Errorf("failed to delete object: %w", err)
	}

	c.log.Debug("MinIO.Delete: файл удалён", "key", objectKey)
	return nil
}

// DeleteMultiple удаляет несколько файлов
func (c *Client) DeleteMultiple(ctx context.Context, objectKeys []string) error {
	objectsCh := make(chan minio.ObjectInfo, len(objectKeys))
	for _, key := range objectKeys {
		objectsCh <- minio.ObjectInfo{Key: key}
	}
	close(objectsCh)

	for err := range c.client.RemoveObjects(ctx, c.bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if err.Err != nil {
			c.log.Error("MinIO.DeleteMultiple: ошибка удаления", "error", err.Err, "key", err.ObjectName)
			return err.Err
		}
	}

	c.log.Debug("MinIO.DeleteMultiple: файлы удалены", "count", len(objectKeys))
	return nil
}

// GetThumbnailKey генерирует ключ для thumbnail
func GetThumbnailKey(objectKey string) string {
	ext := filepath.Ext(objectKey)
	name := objectKey[:len(objectKey)-len(ext)]
	return name + "_thumb" + ext
}

// GetMediumKey генерирует ключ для medium версии
func GetMediumKey(objectKey string) string {
	ext := filepath.Ext(objectKey)
	name := objectKey[:len(objectKey)-len(ext)]
	return name + "_medium" + ext
}

// Close закрывает клиент
func (c *Client) Close() error {
	return nil
}
