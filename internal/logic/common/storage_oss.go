package common

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OSSStorageProvider implements StorageProvider using Aliyun OSS.
type OSSStorageProvider struct {
	client *oss.Client
	bucket *oss.Bucket
	prefix string
}

// NewOSSProvider creates a new Aliyun OSS storage provider.
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

// fullKey returns the full object key with prefix.
func (s *OSSStorageProvider) fullKey(key string) string {
	if s.prefix != "" {
		return s.prefix + "/" + key
	}
	return key
}

// Upload uploads a file to Aliyun OSS and returns the storage key.
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

// Download returns a reader for the file at the given key.
func (s *OSSStorageProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	fullKey := s.fullKey(key)

	resp, err := s.bucket.GetObject(fullKey)
	if err != nil {
		return nil, fmt.Errorf("oss get object: %w", err)
	}

	return resp, nil
}

// Delete deletes a file from Aliyun OSS.
func (s *OSSStorageProvider) Delete(ctx context.Context, key string) error {
	fullKey := s.fullKey(key)

	err := s.bucket.DeleteObject(fullKey)
	if err != nil {
		return fmt.Errorf("oss delete object: %w", err)
	}

	return nil
}

// PresignedURL generates a temporary signed URL for downloading.
func (s *OSSStorageProvider) PresignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	fullKey := s.fullKey(key)

	signedURL, err := s.bucket.SignURL(fullKey, oss.HTTPGet, int64(expires.Seconds()))
	if err != nil {
		return "", fmt.Errorf("oss sign url: %w", err)
	}

	return signedURL, nil
}
