package common

import (
	"context"
	"fmt"
)

// StorageConfig holds the configuration for a storage provider.
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

// GetStorageConfig reads storage configuration from the config service.
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

// NewStorageProvider creates a StorageProvider based on the given configuration.
func NewStorageProvider(cfg *StorageConfig) (StorageProvider, error) {
	switch cfg.Provider {
	case "s3", "minio":
		return NewS3Provider(cfg)
	case "oss":
		return NewOSSProvider(cfg)
	case "cos":
		return NewCOSProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", cfg.Provider)
	}
}

// NewFileServiceFromConfig creates a FileService using the storage configuration
// from the config service (database settings).
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
