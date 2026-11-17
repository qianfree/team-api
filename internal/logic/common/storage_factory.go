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
	LocalDir    string // 本地存储根目录，仅 provider=local 时使用
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
		LocalDir:    Config().GetString(ctx, "storage_local_dir"),
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
	case "local":
		return NewLocalProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", cfg.Provider)
	}
}

// CloudStorageProviders 支持签名 URL 的云对象存储 provider 集合（local 不在其中：
// 其 PresignedURL 直接报错）。relay 文件上传门禁与公开设置标志共用，避免两处口径漂移。
var CloudStorageProviders = map[string]struct{}{
	"s3": {}, "minio": {}, "r2": {}, "oss": {}, "cos": {},
}

// IsStorageConfigured 轻量判断对象存储是否已配置（provider + bucket 齐全）。
// 只读配置、不构造 provider、不产生网络调用，供提交阶段的 fast-fail 使用。
func IsStorageConfigured(ctx context.Context) bool {
	cfg := GetStorageConfig(ctx)
	return cfg.Provider != "" && cfg.Bucket != ""
}

// IsCloudStorageConfigured 判断云对象存储（可签发 URL）是否已配置。
// 与 IsStorageConfigured 的区别：provider=local 不算——local 无法提供签名 URL，
// /v1/files 上传通道（base64 素材转存对象存储再以 URL 引用）依赖该能力。
func IsCloudStorageConfigured(ctx context.Context) bool {
	cfg := GetStorageConfig(ctx)
	_, ok := CloudStorageProviders[cfg.Provider]
	return ok && cfg.Bucket != ""
}

// NewFileServiceFromConfig 使用配置服务（数据库设置）中的存储配置创建一个 FileService。
//
// 对象存储未配置（provider 或 bucket 为空）时，**降级到本地磁盘存储**（provider=local），
// 保证数据导出、文件下载/删除等基本管理功能在未配对象存储时仍可用（仅适合单实例部署）。
// 因此本函数几乎不返回错误，调用方拿到的是「当前可用的存储后端」而非「一定是对象存储」。
//
// 需要强制要求对象存储的链路（如 AI 图片 re-host）应改用 IsStorageConfigured 判断，
// 并自行返回 ErrStorageNotConfigured 哨兵，以保留「未配置」这一可操作语义。
func NewFileServiceFromConfig(ctx context.Context) (*FileService, error) {
	cfg := GetStorageConfig(ctx)
	if cfg.Provider == "" || cfg.Bucket == "" {
		localCfg := *cfg
		localCfg.Provider = "local"
		provider, err := NewStorageProvider(&localCfg)
		if err != nil {
			return nil, err
		}
		return &FileService{provider: provider, providerName: "local"}, nil
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
