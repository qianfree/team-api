package common

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3StorageProvider 基于 AWS S3 / MinIO 实现 StorageProvider。
type S3StorageProvider struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3Provider 创建一个 S3/MinIO 存储 provider。
func NewS3Provider(cfg *StorageConfig) (*S3StorageProvider, error) {
	var opts []func(*awsconfig.LoadOptions) error

	if cfg.Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
			return aws.Endpoint{URL: cfg.Endpoint}, nil
		})
		opts = append(opts, awsconfig.WithEndpointResolverWithOptions(customResolver))
	}

	if cfg.AccessKeyID != "" && cfg.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Region != "" {
			o.Region = cfg.Region
		} else if cfg.Provider == "r2" {
			// Cloudflare R2 的 SigV4 签名要求非空 region，官方约定用 "auto"。
			// 后端在此兜底，避免仅依赖前端 UI 填值——经配置接口/DB 直写或 region 被清空时，
			// 空 region 会导致签名失败。
			o.Region = "auto"
		}
		// Cloudflare R2 兼容 S3，但需要账号级的 BaseEndpoint。与 MinIO 不同，它使用
		// 虚拟主机式寻址（virtual-hosted），因此**不能**强制 path-style。
		if (cfg.Provider == "minio" || cfg.Provider == "r2") && cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			if cfg.Provider == "minio" {
				o.UsePathStyle = true
			}
		}
	})

	return &S3StorageProvider{
		client: client,
		bucket: cfg.Bucket,
		prefix: cfg.PathPrefix,
	}, nil
}

// fullKey 返回带前缀的完整对象 key（对已带前缀的存量 key 幂等）。
func (s *S3StorageProvider) fullKey(key string) string {
	return applyStoragePrefix(s.prefix, key)
}

// Upload 上传文件到 S3/MinIO 并返回存储 key。
func (s *S3StorageProvider) Upload(ctx context.Context, reader io.Reader, key string, contentType string) (string, error) {
	fullKey := s.fullKey(key)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(fullKey),
		Body:        reader,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("s3 put object: %w", err)
	}

	return fullKey, nil
}

// Download 返回指定 key 文件的读取器。
func (s *S3StorageProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	fullKey := s.fullKey(key)

	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get object: %w", err)
	}

	return resp.Body, nil
}

// Delete 从 S3/MinIO 删除文件。
func (s *S3StorageProvider) Delete(ctx context.Context, key string) error {
	fullKey := s.fullKey(key)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return fmt.Errorf("s3 delete object: %w", err)
	}

	return nil
}

// PresignedURL 生成用于下载的临时预签名 URL。
func (s *S3StorageProvider) PresignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	fullKey := s.fullKey(key)

	presignClient := s3.NewPresignClient(s.client)
	req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", fmt.Errorf("s3 presign object: %w", err)
	}

	return req.URL, nil
}

// PresignedThumbnailURL 回退到原图对象：S3/MinIO/R2 没有原生的服务端图片处理能力，
// 无法生成真正的缩略图。
func (s *S3StorageProvider) PresignedThumbnailURL(ctx context.Context, key string, width int, expires time.Duration) (string, error) {
	return s.PresignedURL(ctx, key, expires)
}
