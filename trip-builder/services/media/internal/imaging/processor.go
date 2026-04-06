package imaging

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
)

type ProcessedImage struct {
	Thumbnail []byte
	Medium    []byte
	Width     int
	Height    int
}

// ProcessImage генерирует thumbnail (320x180) и medium (1280x720)
func ProcessImage(data []byte, contentType string) (*ProcessedImage, error) {
	// Декодируем изображение
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()

	// Генерируем thumbnail (320x180, сохраняем пропорции, обрезаем если нужно)
	thumb := imaging.Resize(img, 320, 0, imaging.Lanczos)
	if thumb.Bounds().Dy() > 180 {
		thumb = imaging.CropCenter(thumb, 320, 180)
	}

	// Генерируем medium (1280x720, сохраняем пропорции)
	medium := imaging.Resize(img, 1280, 0, imaging.Lanczos)

	// Кодируем в JPEG
	thumbBuf := new(bytes.Buffer)
	err = imaging.Encode(thumbBuf, thumb, imaging.JPEG, imaging.JPEGQuality(85))
	if err != nil {
		return nil, err
	}

	mediumBuf := new(bytes.Buffer)
	err = imaging.Encode(mediumBuf, medium, imaging.JPEG, imaging.JPEGQuality(85))
	if err != nil {
		return nil, err
	}

	return &ProcessedImage{
		Thumbnail: thumbBuf.Bytes(),
		Medium:    mediumBuf.Bytes(),
		Width:     originalWidth,
		Height:    originalHeight,
	}, nil
}

// EncodeJPEG encodes image to JPEG bytes
func EncodeJPEG(img image.Image, quality int) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EncodePNG encodes image to PNG bytes
func EncodePNG(img image.Image) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GetContentType определяет content-type по расширению или данным
func GetContentType(data []byte) string {
	// Простая проверка сигнатуры
	if len(data) < 12 {
		return "application/octet-stream"
	}

	// JPEG
	if data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}
	// PNG
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}
	// GIF
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}
	// WebP
	if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		return "image/webp"
	}

	return "application/octet-stream"
}

// IsImage проверяет, является ли файл изображением
func IsImage(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

// IsVideo проверяет, является ли файл видео
func IsVideo(contentType string) bool {
	switch contentType {
	case "video/mp4", "video/quicktime", "video/x-msvideo":
		return true
	default:
		return false
	}
}

// GenerateObjectKey генерирует уникальный ключ для объекта
func GenerateObjectKey(prefix string, ext string) string {
	if ext == "" {
		ext = ".jpg"
	}
	if ext[0] != '.' {
		ext = "." + ext
	}
	return prefix + "/" + uuid.New().String() + ext
}
