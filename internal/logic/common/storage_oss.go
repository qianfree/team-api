package common

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OSSStorageProvider 基于阿里云 OSS 实现 StorageProvider。
type OSSStorageProvider struct {
	client *oss.Client
	bucket *oss.Bucket
	prefix string
}

// NewOSSProvider 创建一个阿里云 OSS 存储 provider。
func NewOSSProvider(cfg *StorageConfig) (*OSSStorageProvider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("oss endpoint is required")
	}

	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("create oss client: %w", err)
	}

	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("get oss bucket: %w", err)
	}

	return &OSSStorageProvider{
		client: client,
		bucket: bucket,
		prefix: cfg.PathPrefix,
	}, nil
}

// fullKey 返回带前缀的完整对象 key（对已带前缀的存量 key 幂等）。
func (s *OSSStorageProvider) fullKey(key string) string {
	return applyStoragePrefix(s.prefix, key)
}

// Upload 上传文件到阿里云 OSS 并返回存储 key。
func (s *OSSStorageProvider) Upload(ctx context.Context, reader io.Reader, key string, contentType string) (string, error) {
	fullKey := s.fullKey(key)

	options := []oss.Option{
		oss.ContentType(contentType),
	}
	err := s.bucket.PutObject(fullKey, reader, options...)
	if err != nil {
		return "", fmt.Errorf("oss put object: %w", err)
	}

	return fullKey, nil
}

// Download 返回指定 key 文件的读取器。
func (s *OSSStorageProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	fullKey := s.fullKey(key)

	resp, err := s.bucket.GetObject(fullKey)
	if err != nil {
		return nil, fmt.Errorf("oss get object: %w", err)
	}

	return resp, nil
}

// Delete 从阿里云 OSS 删除文件。
func (s *OSSStorageProvider) Delete(ctx context.Context, key string) error {
	fullKey := s.fullKey(key)

	err := s.bucket.DeleteObject(fullKey)
	if err != nil {
		return fmt.Errorf("oss delete object: %w", err)
	}

	return nil
}

// PresignedURL 生成用于下载的临时签名 URL。
func (s *OSSStorageProvider) PresignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	fullKey := s.fullKey(key)

	signedURL, err := s.bucket.SignURL(fullKey, oss.HTTPGet, int64(expires.Seconds()))
	if err != nil {
		return "", fmt.Errorf("oss sign url: %w", err)
	}

	return signedURL, nil
}

// PresignedThumbnailURL 在签名 URL 中附加 x-oss-process 的 image/resize 处理参数，
// 让 OSS 返回按比例缩小的缩略图（宽度按像素，高度自适应，不放大）。
func (s *OSSStorageProvider) PresignedThumbnailURL(ctx context.Context, key string, width int, expires time.Duration) (string, error) {
	fullKey := s.fullKey(key)

	process := fmt.Sprintf("image/resize,w_%d", width)
	signedURL, err := s.bucket.SignURL(fullKey, oss.HTTPGet, int64(expires.Seconds()), oss.Process(process))
	if err != nil {
		return "", fmt.Errorf("oss sign thumbnail url: %w", err)
	}

	return signedURL, nil
}
