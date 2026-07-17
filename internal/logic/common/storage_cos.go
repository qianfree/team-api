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

// COSStorageProvider implements StorageProvider using Tencent COS.
type COSStorageProvider struct {
	client    *cos.Client
	bucket    string
	prefix    string
	secretID  string
	secretKey string
}

// NewCOSProvider creates a new Tencent COS storage provider.
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

// fullKey returns the full object key with prefix.
func (s *COSStorageProvider) fullKey(key string) string {
	if s.prefix != "" {
		return s.prefix + "/" + key
	}
	return key
}

// Upload uploads a file to Tencent COS and returns the storage key.
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

// Download returns a reader for the file at the given key.
func (s *COSStorageProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	fullKey := s.fullKey(key)

	resp, err := s.client.Object.Get(ctx, fullKey, nil)
	if err != nil {
		return nil, fmt.Errorf("cos get object: %w", err)
	}

	return resp.Body, nil
}

// Delete deletes a file from Tencent COS.
func (s *COSStorageProvider) Delete(ctx context.Context, key string) error {
	fullKey := s.fullKey(key)

	_, err := s.client.Object.Delete(ctx, fullKey)
	if err != nil {
		return fmt.Errorf("cos delete object: %w", err)
	}

	return nil
}

// PresignedURL generates a temporary presigned URL for downloading.
func (s *COSStorageProvider) PresignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	fullKey := s.fullKey(key)

	presignedURL, err := s.client.Object.GetPresignedURL(ctx, http.MethodGet, fullKey, s.secretID, s.secretKey, expires, nil)
	if err != nil {
		return "", fmt.Errorf("cos presign url: %w", err)
	}

	return presignedURL.String(), nil
}

// PresignedThumbnailURL signs a URL with a COS CI imageMogr2/thumbnail op so COS
// returns a downscaled thumbnail (width px, height auto). The process param is
// injected into the signed query so the signature covers it.
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
