package common

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// COSStorageProvider 基于腾讯云 COS 实现 StorageProvider。
type COSStorageProvider struct {
	client    *cos.Client
	bucket    string
	prefix    string
	secretID  string
	secretKey string
}

// NewCOSProvider 创建一个腾讯云 COS 存储 provider。
func NewCOSProvider(cfg *StorageConfig) (*COSStorageProvider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("cos endpoint is required")
	}

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse cos endpoint: %w", err)
	}
	u.Path = "/" + cfg.Bucket

	b := &cos.BaseURL{BucketURL: u}
	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.AccessKeyID,
			SecretKey: cfg.SecretKey,
		},
	})

	return &COSStorageProvider{
		client:    client,
		bucket:    cfg.Bucket,
		prefix:    cfg.PathPrefix,
		secretID:  cfg.AccessKeyID,
		secretKey: cfg.SecretKey,
	}, nil
}

// fullKey 返回带前缀的完整对象 key（对已带前缀的存量 key 幂等）。
func (s *COSStorageProvider) fullKey(key string) string {
	return applyStoragePrefix(s.prefix, key)
}

// Upload 上传文件到腾讯云 COS 并返回存储 key。
func (s *COSStorageProvider) Upload(ctx context.Context, reader io.Reader, key string, contentType string) (string, error) {
	fullKey := s.fullKey(key)

	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: contentType,
		},
	}
	_, err := s.client.Object.Put(ctx, fullKey, reader, opt)
	if err != nil {
		return "", fmt.Errorf("cos put object: %w", err)
	}

	return fullKey, nil
}

// Download 返回指定 key 文件的读取器。
func (s *COSStorageProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	fullKey := s.fullKey(key)

	resp, err := s.client.Object.Get(ctx, fullKey, nil)
	if err != nil {
		return nil, fmt.Errorf("cos get object: %w", err)
	}

	return resp.Body, nil
}

// Delete 从腾讯云 COS 删除文件。
func (s *COSStorageProvider) Delete(ctx context.Context, key string) error {
	fullKey := s.fullKey(key)

	_, err := s.client.Object.Delete(ctx, fullKey)
	if err != nil {
		return fmt.Errorf("cos delete object: %w", err)
	}

	return nil
}

// PresignedURL 生成用于下载的临时预签名 URL。
func (s *COSStorageProvider) PresignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	fullKey := s.fullKey(key)

	presignedURL, err := s.client.Object.GetPresignedURL(ctx, http.MethodGet, fullKey, s.secretID, s.secretKey, expires, nil)
	if err != nil {
		return "", fmt.Errorf("cos presign url: %w", err)
	}

	return presignedURL.String(), nil
}

// PresignedThumbnailURL 在签名 URL 中附加 COS CI 的 imageMogr2/thumbnail 处理参数，
// 让 COS 返回按比例缩小的缩略图（宽度按像素，高度自适应）。处理参数被注入到签名的
// query 中，从而被签名覆盖。
func (s *COSStorageProvider) PresignedThumbnailURL(ctx context.Context, key string, width int, expires time.Duration) (string, error) {
	fullKey := s.fullKey(key)

	q := &url.Values{}
	q.Add(fmt.Sprintf("imageMogr2/thumbnail/%dx", width), "")
	opt := &cos.PresignedURLOptions{Query: q}

	presignedURL, err := s.client.Object.GetPresignedURL(ctx, http.MethodGet, fullKey, s.secretID, s.secretKey, expires, opt)
	if err != nil {
		return "", fmt.Errorf("cos presign thumbnail url: %w", err)
	}

	return presignedURL.String(), nil
}
