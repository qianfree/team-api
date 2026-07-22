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
	"github.com/google/uuid"
)

// StorageProvider 定义文件存储后端的统一接口。
type StorageProvider interface {
	// Upload 上传文件并返回存储路径。
	Upload(ctx context.Context, reader io.Reader, key string, contentType string) (string, error)
	// Download 返回指定路径文件的读取器。
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete 删除文件。
	Delete(ctx context.Context, key string) error
	// PresignedURL 生成用于下载的临时 URL。
	PresignedURL(ctx context.Context, key string, expires time.Duration) (string, error)
	// PresignedThumbnailURL 生成返回缩略图的临时 URL（宽度按像素，高度自适应）。
	// 具备原生服务端图片处理能力的 provider（OSS/COS）在签名 URL 中直接应用缩放；
	// 不具备的 provider（S3/MinIO/R2）回退返回原图对象的 URL。
	PresignedThumbnailURL(ctx context.Context, key string, width int, expires time.Duration) (string, error)
}

// FileService 提供带元数据追踪的文件存储操作。
type FileService struct {
	provider     StorageProvider
	providerName string
}

// NewFileService 用给定的 provider 创建一个 FileService。
func NewFileService(provider StorageProvider) *FileService {
	return &FileService{
		provider:     provider,
		providerName: "unknown",
	}
}

// FileUpload 表示一次文件上传请求。
type FileUpload struct {
	Reader      io.Reader
	Filename    string
	ContentType string
	Size        int64
	TenantID    int64
	UserID      int64
}

// FileRecord 表示一条文件元数据记录。
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

// applyStoragePrefix 幂等地为原始 key 拼接配置的路径前缀。
//
// 兼容性关键：早期版本的 FileService.Upload 持久化的是 provider 返回的**已带前缀** key
// （如 team-api/42/..），现已改为持久化 raw key（42/..）并在读写时由 provider 重新加前缀。
// 若对存量行再无条件加一次前缀，会得到 team-api/team-api/42/.. → 删除/下载/缩略图全部失效。
// 因此这里对「已带前缀」的 key 原样返回，使 Delete/PresignedURL/Download 对旧行（带前缀）与
// 新行（raw）都正确，无需数据迁移。raw key 以数字 tenant_id 开头，绝不会与前缀名冲突。
func applyStoragePrefix(prefix, key string) string {
	if prefix == "" {
		return key
	}
	if key == prefix || strings.HasPrefix(key, prefix+"/") {
		return key
	}
	return prefix + "/" + key
}

// Upload 上传文件、通过 provider 存储，并写入元数据记录。
func (s *FileService) Upload(ctx context.Context, upload *FileUpload) (*FileRecord, error) {
	// 生成存储 key（相对于 provider 配置的路径前缀）：{日期}/{uuid}{扩展名}——
	// 按日期分目录便于生命周期策略，UUID 保证唯一且不可猜测，保留原始扩展名。
	ext := extFromFilename(upload.Filename)
	uuidStr := uuid.New().String()
	key := fmt.Sprintf("%s/%s%s", time.Now().Format("2006-01-02"), uuidStr, ext)
	fileName := uuidStr + ext

	// 上传到存储 provider。provider 内部会应用配置的路径前缀，因此这里持久化 raw key
	// （而非返回的 full key）：Download/Delete/PresignedURL 会把该 key 再次传回 provider，
	// 由其恰好再加一次前缀。若在此存储已带前缀的 key，读取时会被重复加前缀两次。
	if _, err := s.provider.Upload(ctx, upload.Reader, key, upload.ContentType); err != nil {
		return nil, gerror.Wrapf(err, "upload to storage")
	}

	// 写入元数据记录
	result, err := dao.FilFiles.Ctx(ctx).Data(do.FilFiles{
		TenantId:        upload.TenantID,
		UserId:          upload.UserID,
		Filename:        fileName,
		OriginalName:    upload.Filename,
		MimeType:        upload.ContentType,
		Size:            upload.Size,
		StorageProvider: s.providerName,
		StoragePath:     key,
		VirusScanStatus: "pending",
	}).Insert()
	if err != nil {
		// 元数据写入失败，尝试清理已上传的文件
		_ = s.provider.Delete(ctx, key)
		return nil, gerror.Wrapf(err, "insert file record")
	}

	id, _ := result.LastInsertId()

	return &FileRecord{
		ID:              id,
		TenantID:        upload.TenantID,
		UserID:          upload.UserID,
		Filename:        fileName,
		OriginalName:    upload.Filename,
		MimeType:        upload.ContentType,
		Size:            upload.Size,
		StorageProvider: s.providerName,
		StoragePath:     key,
		VirusScanStatus: "pending",
	}, nil
}

// GetDownloadURL 生成用于下载文件的预签名 URL。
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

// GetThumbnailURL 生成用于预览缩略图的预签名 URL。只有图片会做服务端缩放；
// 非图片文件（以及不支持原生图片处理的 provider）回退到原图对象。
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
	// 非图片对象无法缩放；返回原图的预签名 URL。
	if !strings.HasPrefix(record.MimeType, "image/") {
		return s.provider.PresignedURL(ctx, record.StoragePath, 24*time.Hour)
	}
	return s.provider.PresignedThumbnailURL(ctx, record.StoragePath, width, 24*time.Hour)
}

// Delete 从存储中删除文件并删除其元数据记录。
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

	// 从存储删除对象。删除失败时**不**继续删库行——否则对象会成为孤儿且失去 DB 索引，
	// 之后再也无法定位清理。保留库行并返回错误，交由调用方（保留期清理循环）下一轮重试。
	// S3/OSS/COS 删除不存在的对象均返回成功，故此处的错误代表真实的瞬时/权限问题，值得重试。
	if err := s.provider.Delete(ctx, record.StoragePath); err != nil {
		return gerror.Wrapf(err, "delete file %d from storage", fileID)
	}

	// 删除记录
	_, err = dao.FilFiles.Ctx(ctx).
		Where("id", fileID).
		Delete()
	if err != nil {
		return gerror.Wrapf(err, "delete file record")
	}

	return nil
}

// extFromFilename 从文件名中提取扩展名（含前导点）。无扩展名时返回空字符串。
func extFromFilename(name string) string {
	if idx := strings.LastIndex(name, "."); idx > 0 {
		return name[idx:]
	}
	return ""
}
