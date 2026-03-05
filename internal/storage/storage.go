// Package storage реализует работу с объектным хранилищем (MinIO / S3).
// Предоставляет interface-driven подход для загрузки и удаления файлов.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ────────────────────────────────────────────────────────────────────────────
//  Константы
// ────────────────────────────────────────────────────────────────────────────

const (
	// BucketLogos — бакет для логотипов вузов (публичный READ-ONLY).
	BucketLogos = "logos"

	// BucketDocuments — бакет для внутренних документов (приватный).
	BucketDocuments = "documents"

	// maxLogoSize — максимальный размер логотипа (5 МБ).
	maxLogoSize = 5 << 20
)

// allowedImageTypes — допустимые MIME-типы для загрузки изображений.
var allowedImageTypes = map[string]bool{
	"image/jpeg":    true,
	"image/png":     true,
	"image/webp":    true,
	"image/svg+xml": true,
}

// ────────────────────────────────────────────────────────────────────────────
//  Конфигурация
// ────────────────────────────────────────────────────────────────────────────

// Config хранит параметры подключения к MinIO / S3.
type Config struct {
	// Endpoint — адрес MinIO без схемы (например, "localhost:9000").
	Endpoint string

	// AccessKey — ключ доступа (MINIO_ROOT_USER).
	AccessKey string

	// SecretKey — секретный ключ (MINIO_ROOT_PASSWORD).
	SecretKey string

	// UseSSL — использовать HTTPS при подключении к MinIO.
	UseSSL bool

	// PublicEndpoint — внешний URL, по которому клиенты обращаются к файлам.
	// Например: "http://localhost:9000" или "https://cdn.example.com".
	// Используется для формирования публичных ссылок на загруженные объекты.
	PublicEndpoint string
}

// ────────────────────────────────────────────────────────────────────────────
//  Интерфейс
// ────────────────────────────────────────────────────────────────────────────

// Uploader описывает контракт для работы с файловым хранилищем.
// Интерфейс позволяет подменять реализацию (например, для тестов).
type Uploader interface {
	// UploadImage загружает изображение (логотип) в бакет logos.
	// Принимает *multipart.FileHeader из Fiber-хендлера.
	// Возвращает публичный URL загруженного файла.
	//
	// Выполняет валидацию:
	//   - Content-Type должен быть допустимым изображением;
	//   - размер файла не превышает maxLogoSize (5 МБ).
	UploadImage(ctx context.Context, file *multipart.FileHeader) (string, error)

	// UploadDocument загружает произвольный документ в бакет documents.
	// Возвращает путь (ключ объекта) для сохранения в базе данных.
	UploadDocument(ctx context.Context, file *multipart.FileHeader) (string, error)

	// DeleteFile удаляет файл из указанного бакета.
	// fileName — полный ключ объекта (например, "a1b2c3d4.png").
	// bucket — имя бакета (BucketLogos или BucketDocuments).
	DeleteFile(ctx context.Context, bucket, fileName string) error
}

// ────────────────────────────────────────────────────────────────────────────
//  Реализация (MinIO)
// ────────────────────────────────────────────────────────────────────────────

// minioStorage — реализация Uploader поверх MinIO.
type minioStorage struct {
	client         *minio.Client
	publicEndpoint string
}

// NewMinioStorage создаёт MinIO-клиент, проверяет / создаёт необходимые
// бакеты и настраивает политику доступа.
//
// Эта функция должна вызываться при старте приложения (fail-fast).
func NewMinioStorage(ctx context.Context, cfg Config) (Uploader, error) {
	// 1. Создаём клиент MinIO.
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: minio client: %w", err)
	}

	// 2. Проверяем доступность MinIO (health check через list buckets).
	_, err = client.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage: minio unreachable at %s: %w", cfg.Endpoint, err)
	}
	log.Printf("[storage] connected to MinIO at %s", cfg.Endpoint)

	s := &minioStorage{
		client:         client,
		publicEndpoint: strings.TrimRight(cfg.PublicEndpoint, "/"),
	}

	// 3. Инициализация бакетов.
	if err := s.ensureBuckets(ctx); err != nil {
		return nil, err
	}

	return s, nil
}

// ensureBuckets проверяет наличие обязательных бакетов и создаёт
// отсутствующие. Для logos устанавливает публичную READ-ONLY политику.
func (s *minioStorage) ensureBuckets(ctx context.Context) error {
	buckets := []string{BucketLogos, BucketDocuments}

	for _, bucket := range buckets {
		exists, err := s.client.BucketExists(ctx, bucket)
		if err != nil {
			return fmt.Errorf("storage: check bucket %q: %w", bucket, err)
		}

		if !exists {
			if err := s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return fmt.Errorf("storage: create bucket %q: %w", bucket, err)
			}
			log.Printf("[storage] bucket %q created", bucket)
		} else {
			log.Printf("[storage] bucket %q already exists", bucket)
		}
	}

	// Устанавливаем READ-ONLY политику для бакета logos.
	if err := s.setLogosPublicReadPolicy(ctx); err != nil {
		return err
	}

	return nil
}

// setLogosPublicReadPolicy устанавливает S3-совместимую bucket policy,
// разрешающую анонимный GET (s3:GetObject) для всех объектов в бакете logos.
// Загрузка (PutObject) остаётся доступной только аутентифицированным
// клиентам (нашему бэкенду).
func (s *minioStorage) setLogosPublicReadPolicy(ctx context.Context) error {
	policy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect":    "Allow",
				"Principal": map[string]any{"AWS": []string{"*"}},
				"Action":    []string{"s3:GetObject"},
				"Resource":  []string{fmt.Sprintf("arn:aws:s3:::%s/*", BucketLogos)},
			},
		},
	}

	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("storage: marshal logos policy: %w", err)
	}

	if err := s.client.SetBucketPolicy(ctx, BucketLogos, string(policyJSON)); err != nil {
		return fmt.Errorf("storage: set logos policy: %w", err)
	}

	log.Printf("[storage] bucket %q: public READ-ONLY policy applied", BucketLogos)
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
//  UploadImage
// ────────────────────────────────────────────────────────────────────────────

func (s *minioStorage) UploadImage(ctx context.Context, file *multipart.FileHeader) (string, error) {
	// 1. Валидация размера.
	if file.Size > maxLogoSize {
		return "", fmt.Errorf("storage: file too large: %d bytes (max %d)", file.Size, maxLogoSize)
	}

	// 2. Определяем и валидируем Content-Type.
	contentType, err := detectContentType(file)
	if err != nil {
		return "", err
	}

	if !allowedImageTypes[contentType] {
		return "", fmt.Errorf("storage: unsupported image type %q; allowed: jpeg, png, webp, svg", contentType)
	}

	// 3. Генерируем уникальное имя файла.
	ext := normalizeExt(filepath.Ext(file.Filename), contentType)
	objectName := uuid.New().String() + ext

	// 4. Открываем файл для чтения.
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("storage: open uploaded file: %w", err)
	}
	defer func() { _ = src.Close() }()

	// 5. Загружаем в MinIO.
	_, err = s.client.PutObject(ctx, BucketLogos, objectName, src, file.Size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("storage: upload image to %s/%s: %w", BucketLogos, objectName, err)
	}

	// 6. Формируем публичный URL.
	publicURL := fmt.Sprintf("%s/%s/%s", s.publicEndpoint, BucketLogos, objectName)

	log.Printf("[storage] image uploaded: %s (%d bytes, %s)", objectName, file.Size, contentType)
	return publicURL, nil
}

// ────────────────────────────────────────────────────────────────────────────
//  UploadDocument
// ────────────────────────────────────────────────────────────────────────────

func (s *minioStorage) UploadDocument(ctx context.Context, file *multipart.FileHeader) (string, error) {
	// Определяем Content-Type.
	contentType, err := detectContentType(file)
	if err != nil {
		return "", err
	}

	ext := normalizeExt(filepath.Ext(file.Filename), contentType)
	objectName := uuid.New().String() + ext

	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("storage: open uploaded file: %w", err)
	}
	defer func() { _ = src.Close() }()

	_, err = s.client.PutObject(ctx, BucketDocuments, objectName, src, file.Size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("storage: upload document to %s/%s: %w", BucketDocuments, objectName, err)
	}

	// Для приватных документов возвращаем только ключ объекта (не публичный URL).
	log.Printf("[storage] document uploaded: %s (%d bytes, %s)", objectName, file.Size, contentType)
	return objectName, nil
}

// ────────────────────────────────────────────────────────────────────────────
//  DeleteFile
// ────────────────────────────────────────────────────────────────────────────

func (s *minioStorage) DeleteFile(ctx context.Context, bucket, fileName string) error {
	if bucket == "" || fileName == "" {
		return fmt.Errorf("storage: bucket and fileName must not be empty")
	}

	err := s.client.RemoveObject(ctx, bucket, fileName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("storage: delete %s/%s: %w", bucket, fileName, err)
	}

	log.Printf("[storage] file deleted: %s/%s", bucket, fileName)
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
//  Helpers
// ────────────────────────────────────────────────────────────────────────────

// detectContentType определяет MIME-тип файла, читая первые 512 байт.
// Не полагается на заголовок Content-Type из multipart-формы, поскольку
// браузеры могут указывать некорректный тип.
func detectContentType(fh *multipart.FileHeader) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("storage: open file for content detection: %w", err)
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("storage: read file header: %w", err)
	}

	ct := http.DetectContentType(buf[:n])

	// http.DetectContentType не распознаёт SVG (возвращает text/xml или text/plain).
	// Если расширение .svg — доверяем ему.
	if strings.HasSuffix(strings.ToLower(fh.Filename), ".svg") {
		ct = "image/svg+xml"
	}

	return ct, nil
}

// normalizeExt возвращает расширение файла. Если исходное расширение
// пустое или не соответствует Content-Type, подбирает расширение из MIME.
func normalizeExt(ext, contentType string) string {
	ext = strings.ToLower(ext)
	if ext != "" {
		return ext
	}

	// Фоллбэк по Content-Type.
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "application/pdf":
		return ".pdf"
	default:
		return ".bin"
	}
}
