package common

import (
	"context"
	"errors"
	"fmt"
)

// ErrStorageNotConfigured 表示对象存储尚未配置（缺少 provider / bucket）。
//
// 用哨兵错误而非普通 error，便于上层（如 sync_image worker）用 errors.Is 精确识别「未配置」
// 这一**可操作**场景，给出面向用户的友好提示（引导去系统设置配置 OSS），从而与上传/下载
// 等技术性失败区分开。
var ErrStorageNotConfigured = errors.New("object storage not configured")

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

// IsStorageConfigured 轻量判断对象存储是否已配置（provider + bucket 齐全）。
// 只读配置、不构造 provider、不产生网络调用，供提交阶段的 fast-fail 使用。
func IsStorageConfigured(ctx context.Context) bool {
	cfg := GetStorageConfig(ctx)
	return cfg.Provider != "" && cfg.Bucket != ""
}

// NewFileServiceFromConfig 使用配置服务（数据库设置）中的存储配置创建一个 FileService。
// 未配置（缺 provider 或 bucket）时返回 ErrStorageNotConfigured 哨兵错误，供上层友好提示。
func NewFileServiceFromConfig(ctx context.Context) (*FileService, error) {
	cfg := GetStorageConfig(ctx)
	if cfg.Provider == "" || cfg.Bucket == "" {
		return nil, ErrStorageNotConfigured
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
