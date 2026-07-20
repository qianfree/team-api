package common

import (
	"context"
	"fmt"
)

// StorageConfig 保存某个存储 provider 的配置。
type StorageConfig struct {
	Provider    string
	Endpoint    string
	Region      string
	Bucket      string
	AccessKeyID string
	SecretKey   string
	UseSSL      bool
	PathPrefix  string
}

// GetStorageConfig 从配置服务读取存储配置。
func GetStorageConfig(ctx context.Context) *StorageConfig {
	cfg := &StorageConfig{
		Provider:    Config().GetString(ctx, "storage_provider"),
		Endpoint:    Config().GetString(ctx, "storage_endpoint"),
		Region:      Config().GetString(ctx, "storage_region"),
		Bucket:      Config().GetString(ctx, "storage_bucket"),
		AccessKeyID: Config().GetString(ctx, "storage_access_key_id"),
		SecretKey:   Config().GetString(ctx, "storage_access_key_secret"),
		UseSSL:      Config().GetBool(ctx, "storage_use_ssl"),
		PathPrefix:  Config().GetString(ctx, "storage_path_prefix"),
	}
	if cfg.PathPrefix == "" {
		cfg.PathPrefix = "team-api"
	}
	return cfg
}

// NewStorageProvider 根据给定配置创建对应的 StorageProvider。
func NewStorageProvider(cfg *StorageConfig) (StorageProvider, error) {
	switch cfg.Provider {
	case "s3", "minio", "r2":
		return NewS3Provider(cfg)
	case "oss":
		return NewOSSProvider(cfg)
	case "cos":
		return NewCOSProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", cfg.Provider)
	}
}

// NewFileServiceFromConfig 使用配置服务（数据库设置）中的存储配置创建一个 FileService。
func NewFileServiceFromConfig(ctx context.Context) (*FileService, error) {
	cfg := GetStorageConfig(ctx)
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("storage bucket not configured")
	}

	provider, err := NewStorageProvider(cfg)
	if err != nil {
		return nil, err
	}

	return &FileService{
		provider:     provider,
		providerName: cfg.Provider,
	}, nil
}
