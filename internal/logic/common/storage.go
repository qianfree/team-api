package common

import (
	"context"
	"fmt"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// StorageProvider defines the interface for file storage backends.
type StorageProvider interface {
	// Upload uploads a file and returns the storage path.
	Upload(ctx context.Context, reader io.Reader, key string, contentType string) (string, error)
	// Download returns a reader for the file at the given path.
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete deletes a file.
	Delete(ctx context.Context, key string) error
	// PresignedURL generates a temporary URL for downloading.
	PresignedURL(ctx context.Context, key string, expires time.Duration) (string, error)
	// PresignedThumbnailURL generates a temporary URL that returns a resized
	// thumbnail (width in pixels, height auto). Providers with native
	// server-side image processing (OSS/COS) apply it in the signed URL;
	// providers without it (S3/MinIO/R2) return the original-object URL.
	PresignedThumbnailURL(ctx context.Context, key string, width int, expires time.Duration) (string, error)
}

// FileService provides file storage operations with metadata tracking.
type FileService struct {
	provider     StorageProvider
	providerName string
}

// NewFileService creates a new FileService with the given provider.
func NewFileService(provider StorageProvider) *FileService {
	return &FileService{
		provider:     provider,
		providerName: "unknown",
	}
}

// FileUpload represents a file upload request.
type FileUpload struct {
	Reader      io.Reader
	Filename    string
	ContentType string
	Size        int64
	TenantID    int64
	UserID      int64
}

// FileRecord represents a file metadata record.
type FileRecord struct {
	ID              int64  `json:"id"`
	TenantID        int64  `json:"tenant_id"`
	UserID          int64  `json:"user_id"`
	Filename        string `json:"filename"`
	OriginalName    string `json:"original_name"`
	MimeType        string `json:"mime_type"`
	Size            int64  `json:"size"`
	StorageProvider string `json:"storage_provider"`
	StoragePath     string `json:"storage_path"`
	VirusScanStatus string `json:"virus_scan_status"`
	Checksum        string `json:"checksum"`
	CreatedAt       string `json:"created_at"`
}

// Upload uploads a file, stores it via the provider, and records metadata.
func (s *FileService) Upload(ctx context.Context, upload *FileUpload) (*FileRecord, error) {
	// Generate storage key (relative to the provider's configured path prefix).
	key := fmt.Sprintf("%d/%d/%s", upload.TenantID, time.Now().Unix(), upload.Filename)

	// Upload to storage provider. The provider applies the configured path
	// prefix internally, so we persist the RAW key (not the returned full key):
	// Download/Delete/PresignedURL pass this key back through the provider,
	// which re-applies the prefix exactly once. Storing the prefixed key here
	// would cause the prefix to be applied twice on retrieval.
	if _, err := s.provider.Upload(ctx, upload.Reader, key, upload.ContentType); err != nil {
		return nil, gerror.Wrapf(err, "upload to storage")
	}

	// Record metadata
	result, err := dao.FilFiles.Ctx(ctx).Data(do.FilFiles{
		TenantId:        upload.TenantID,
		UserId:          upload.UserID,
		Filename:        key,
		OriginalName:    upload.Filename,
		MimeType:        upload.ContentType,
		Size:            upload.Size,
		StorageProvider: s.providerName,
		StoragePath:     key,
		VirusScanStatus: "pending",
	}).Insert()
	if err != nil {
		// Try to clean up the uploaded file
		_ = s.provider.Delete(ctx, key)
		return nil, gerror.Wrapf(err, "insert file record")
	}

	id, _ := result.LastInsertId()

	return &FileRecord{
		ID:              id,
		TenantID:        upload.TenantID,
		UserID:          upload.UserID,
		Filename:        key,
		OriginalName:    upload.Filename,
		MimeType:        upload.ContentType,
		Size:            upload.Size,
		StorageProvider: s.providerName,
		StoragePath:     key,
		VirusScanStatus: "pending",
	}, nil
}

// GetDownloadURL generates a presigned URL for downloading a file.
func (s *FileService) GetDownloadURL(ctx context.Context, fileID int64) (string, error) {
	var record *FileRecord
	err := dao.FilFiles.Ctx(ctx).
		Where("id", fileID).
		Scan(&record)
	if err != nil {
		return "", gerror.Wrapf(err, "query file %d", fileID)
	}
	if record == nil {
		return "", gerror.Newf("file not found: %d", fileID)
	}

	return s.provider.PresignedURL(ctx, record.StoragePath, 24*time.Hour)
}

// GetThumbnailURL generates a presigned URL for previewing a downscaled
// thumbnail. Only images get server-side resizing; non-image files (and
// providers without native image processing) fall back to the original object.
func (s *FileService) GetThumbnailURL(ctx context.Context, fileID int64, width int) (string, error) {
	var record *FileRecord
	err := dao.FilFiles.Ctx(ctx).
		Where("id", fileID).
		Scan(&record)
	if err != nil {
		return "", gerror.Wrapf(err, "query file %d", fileID)
	}
	if record == nil {
		return "", gerror.Newf("file not found: %d", fileID)
	}

	if width <= 0 {
		width = 400
	}
	// Non-image objects cannot be resized; return the original presigned URL.
	if !strings.HasPrefix(record.MimeType, "image/") {
		return s.provider.PresignedURL(ctx, record.StoragePath, 24*time.Hour)
	}
	return s.provider.PresignedThumbnailURL(ctx, record.StoragePath, width, 24*time.Hour)
}

// Delete deletes a file from storage and marks it as deleted.
func (s *FileService) Delete(ctx context.Context, fileID int64) error {
	var record *FileRecord
	err := dao.FilFiles.Ctx(ctx).
		Where("id", fileID).
		Scan(&record)
	if err != nil {
		return gerror.Wrapf(err, "query file %d", fileID)
	}
	if record == nil {
		return gerror.Newf("file not found: %d", fileID)
	}

	// Delete from storage
	if err := s.provider.Delete(ctx, record.StoragePath); err != nil {
		g.Log().Warningf(ctx, "delete file from storage: %v", err)
	}

	// Delete record
	_, err = dao.FilFiles.Ctx(ctx).
		Where("id", fileID).
		Delete()
	if err != nil {
		return gerror.Wrapf(err, "delete file record")
	}

	return nil
}
