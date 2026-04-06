package service

import (
	"context"
	"errors"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/imaging"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/media/internal/minio"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/google/uuid"
)

var (
	ErrInvalidFileType = errors.New("invalid file type")
	ErrFileTooLarge    = errors.New("file too large")
	ErrMediaNotFound   = errors.New("media not found")
	ErrNotAuthorized   = errors.New("not authorized")
)

type MediaService struct {
	repo         repository.MediaRepository
	minio        *minio.Client
	producer     *kafka.MediaProducer
	log          *logger.Logger
	maxImageSize int64
	maxVideoSize int64
}

func NewMediaService(
	repo repository.MediaRepository,
	minioClient *minio.Client,
	producer *kafka.MediaProducer,
	log *logger.Logger,
	maxImageSize, maxVideoSize int64,
) *MediaService {
	return &MediaService{
		repo:         repo,
		minio:        minioClient,
		producer:     producer,
		log:          log,
		maxImageSize: maxImageSize,
		maxVideoSize: maxVideoSize,
	}
}

// Upload загружает файл в MinIO и сохраняет метаданные
func (s *MediaService) Upload(ctx context.Context, userID uuid.UUID, fileData []byte, filename, purpose string, eventID, waypointID *uuid.UUID) (*models.Media, error) {
	s.log.Debug("Upload: начало", "user_id", userID, "filename", filename, "purpose", purpose, "size", len(fileData))

	// Определяем content-type
	contentType := imaging.GetContentType(fileData)
	if contentType == "application/octet-stream" {
		s.log.Debug("Upload: неопределённый тип файла", "filename", filename)
		return nil, ErrInvalidFileType
	}

	// Проверяем тип и размер
	isImage := imaging.IsImage(contentType)
	isVideo := imaging.IsVideo(contentType)

	if !isImage && !isVideo {
		s.log.Debug("Upload: неподдерживаемый тип", "content_type", contentType)
		return nil, ErrInvalidFileType
	}

	// Проверяем размер
	fileSize := int64(len(fileData))
	if isImage && fileSize > s.maxImageSize {
		s.log.Debug("Upload: изображение слишком большое", "size", fileSize, "max", s.maxImageSize)
		return nil, ErrFileTooLarge
	}
	if isVideo && fileSize > s.maxVideoSize {
		s.log.Debug("Upload: видео слишком большое", "size", fileSize, "max", s.maxVideoSize)
		return nil, ErrFileTooLarge
	}

	// Генерируем ключ для MinIO
	var objectKey string
	switch purpose {
	case "avatar":
		objectKey = s.minio.GenerateObjectKey(purpose, userID, filename)
	case "route_image":
		objectKey = s.minio.GenerateObjectKey(purpose, userID, filename)
	case "event_photo":
		if eventID != nil {
			objectKey = s.minio.GenerateObjectKeyForEvent(*eventID, userID, filename)
		} else {
			objectKey = s.minio.GenerateObjectKey(purpose, userID, filename)
		}
	case "waypoint_photo":
		if eventID != nil && waypointID != nil {
			objectKey = s.minio.GenerateObjectKeyForWaypoint(*eventID, *waypointID, userID, filename)
		} else {
			objectKey = s.minio.GenerateObjectKey(purpose, userID, filename)
		}
	default:
		objectKey = s.minio.GenerateObjectKey("general", userID, filename)
	}

	s.log.Trace("Upload: сгенерирован object_key", "object_key", objectKey)

	// Загружаем оригинал в MinIO
	_, err := s.minio.Upload(ctx, objectKey, fileData, contentType)
	if err != nil {
		s.log.Error("Upload: ошибка загрузки в MinIO", "error", err)
		return nil, err
	}

	// Получаем размеры изображения (если это изображение)
	var width, height int
	if isImage {
		processed, err := imaging.ProcessImage(fileData, contentType)
		if err == nil {
			width = processed.Width
			height = processed.Height

			// Асинхронно генерируем и загружаем thumbnail и medium
			go func() {
				ctxBg := context.Background()
				s.processAndUploadThumbnails(ctxBg, objectKey, fileData, contentType)
			}()
		} else {
			s.log.Error("Upload: ошибка обработки изображения", "error", err)
		}
	}

	// Сохраняем метаданные в БД
	media := &models.Media{
		ID:         uuid.New(),
		UserID:     userID,
		EventID:    eventID,
		WaypointID: waypointID,
		Purpose:    purpose,
		Filename:   filename,
		ObjectKey:  objectKey,
		MediaType:  contentType,
		FileSize:   fileSize,
		Width:      width,
		Height:     height,
		CreatedAt:  time.Now(),
	}

	err = s.repo.Create(ctx, media)
	if err != nil {
		s.log.Error("Upload: ошибка сохранения в БД", "error", err)
		// Пытаемся удалить файл из MinIO
		_ = s.minio.Delete(ctx, objectKey)
		return nil, err
	}

	// Публикуем событие
	go func() {
		if err := s.producer.PublishMediaUploaded(context.Background(), media.ID, userID, eventID, waypointID, purpose); err != nil {
			s.log.Error("Upload: ошибка публикации события", "error", err)
		}
	}()

	s.log.Info("Upload: файл загружен", "media_id", media.ID, "object_key", objectKey)
	return media, nil
}

// processAndUploadThumbnails генерирует и загружает thumbnail и medium версии
func (s *MediaService) processAndUploadThumbnails(ctx context.Context, objectKey string, fileData []byte, contentType string) {
	s.log.Debug("processAndUploadThumbnails: начало", "object_key", objectKey)

	processed, err := imaging.ProcessImage(fileData, contentType)
	if err != nil {
		s.log.Error("processAndUploadThumbnails: ошибка обработки", "error", err, "object_key", objectKey)
		return
	}

	// Находим медиа в БД по object_key
	media, err := s.repo.GetByObjectKey(ctx, objectKey)
	if err != nil || media == nil {
		s.log.Error("processAndUploadThumbnails: медиа не найдено", "error", err, "object_key", objectKey)
		return
	}

	var thumbnailURL, mediumURL string

	// Загружаем thumbnail
	thumbKey := minio.GetThumbnailKey(objectKey)
	thumbResult, err := s.minio.Upload(ctx, thumbKey, processed.Thumbnail, "image/jpeg")
	if err != nil {
		s.log.Error("processAndUploadThumbnails: ошибка загрузки thumbnail", "error", err)
	} else {
		thumbnailURL = thumbResult.Key
		s.log.Debug("processAndUploadThumbnails: thumbnail загружен", "key", thumbKey)
	}

	// Загружаем medium
	mediumKey := minio.GetMediumKey(objectKey)
	mediumResult, err := s.minio.Upload(ctx, mediumKey, processed.Medium, "image/jpeg")
	if err != nil {
		s.log.Error("processAndUploadThumbnails: ошибка загрузки medium", "error", err)
	} else {
		mediumURL = mediumResult.Key
		s.log.Debug("processAndUploadThumbnails: medium загружен", "key", mediumKey)
	}

	// Обновляем запись в БД (сохраняем ключи thumbnail и medium)
	if thumbnailURL != "" || mediumURL != "" {
		if err := s.repo.UpdateThumbnails(ctx, media.ID, thumbnailURL, mediumURL); err != nil {
			s.log.Error("processAndUploadThumbnails: ошибка обновления БД", "error", err)
		} else {
			s.log.Debug("processAndUploadThumbnails: БД обновлена", "media_id", media.ID)
		}
	}
}

// GetFile возвращает файл из MinIO по object_key
func (s *MediaService) GetFile(ctx context.Context, objectKey string) ([]byte, string, error) {
	s.log.Debug("GetFile: начало", "object_key", objectKey)
	return s.minio.Download(ctx, objectKey)
}

// GetByID возвращает медиа по ID
func (s *MediaService) GetByID(ctx context.Context, mediaID uuid.UUID) (*models.Media, error) {
	s.log.Debug("GetByID: начало", "media_id", mediaID)

	media, err := s.repo.GetByID(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	if media == nil {
		return nil, ErrMediaNotFound
	}

	return media, nil
}

// GetByEventID возвращает медиа поездки
func (s *MediaService) GetByEventID(ctx context.Context, eventID uuid.UUID, page, size int) ([]models.Media, int64, error) {
	s.log.Debug("GetByEventID: начало", "event_id", eventID, "page", page, "size", size)

	offset := (page - 1) * size
	return s.repo.GetByEventID(ctx, eventID, size, offset)
}

// GetByWaypointID возвращает медиа точки
func (s *MediaService) GetByWaypointID(ctx context.Context, waypointID uuid.UUID, page, size int) ([]models.Media, int64, error) {
	s.log.Debug("GetByWaypointID: начало", "waypoint_id", waypointID, "page", page, "size", size)

	offset := (page - 1) * size
	return s.repo.GetByWaypointID(ctx, waypointID, size, offset)
}

// Delete удаляет медиа
func (s *MediaService) Delete(ctx context.Context, mediaID, userID uuid.UUID) error {
	s.log.Debug("Delete: начало", "media_id", mediaID, "user_id", userID)

	// Проверяем, существует ли медиа
	media, err := s.repo.GetByID(ctx, mediaID)
	if err != nil {
		return err
	}
	if media == nil {
		return ErrMediaNotFound
	}

	// Проверяем права: только владелец может удалить
	if media.UserID != userID {
		s.log.Warn("Delete: попытка удаления чужого медиа", "media_id", mediaID, "user_id", userID, "owner_id", media.UserID)
		return ErrNotAuthorized
	}

	// Удаляем из MinIO по object_key
	if media.ObjectKey != "" {
		if err := s.minio.Delete(ctx, media.ObjectKey); err != nil {
			s.log.Error("Delete: ошибка удаления из MinIO", "error", err, "object_key", media.ObjectKey)
			// Не возвращаем ошибку, продолжаем удаление из БД
		} else {
			s.log.Debug("Delete: оригинал удалён из MinIO", "object_key", media.ObjectKey)
		}

		// Удаляем thumbnail и medium, если есть
		thumbKey := minio.GetThumbnailKey(media.ObjectKey)
		mediumKey := minio.GetMediumKey(media.ObjectKey)
		_ = s.minio.Delete(ctx, thumbKey)
		_ = s.minio.Delete(ctx, mediumKey)
	}

	// Удаляем из БД
	if err := s.repo.Delete(ctx, mediaID); err != nil {
		s.log.Error("Delete: ошибка удаления из БД", "error", err)
		return err
	}

	s.log.Info("Delete: медиа удалено", "media_id", mediaID)
	return nil
}
