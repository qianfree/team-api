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

// S3StorageProvider implements StorageProvider using AWS S3 / MinIO.
type S3StorageProvider struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3Provider creates a new S3/MinIO storage provider.
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
		}
		// Cloudflare R2 is S3-compatible but requires an account-level
		// BaseEndpoint. Unlike MinIO it uses virtual-hosted addressing, so
		// path-style must NOT be forced.
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

// fullKey returns the full object key with prefix.
func (s *S3StorageProvider) fullKey(key string) string {
	if s.prefix != "" {
		return s.prefix + "/" + key
	}
	return key
}

// Upload uploads a file to S3/MinIO and returns the storage key.
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

// Download returns a reader for the file at the given key.
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

// Delete deletes a file from S3/MinIO.
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

// PresignedURL generates a temporary presigned URL for downloading.
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
