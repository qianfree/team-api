package common

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStorageProvider 基于本地磁盘实现 StorageProvider，作为对象存储未配置时的降级后端。
//
// 仅适合单实例部署——多实例间本地磁盘不共享，文件在生成实例之外的副本上会 404。
// 生产环境应配置 OSS/S3/COS；本 provider 主要服务于开发环境与未配对象存储时的基本可用性
// （数据导出、文件下载/删除等），不应用于 AI 图片 re-host 等高频读路径（后者由调用方用
// IsStorageConfigured 守卫，保持「要求对象存储」语义）。
type LocalStorageProvider struct {
	rootDir string // 本地存储根目录（绝对路径）
	prefix  string // 路径前缀，与 OSS provider 对称（默认 team-api）
}

// NewLocalProvider 创建一个本地磁盘存储 provider。rootDir 不存在时会递归创建。
func NewLocalProvider(cfg *StorageConfig) (*LocalStorageProvider, error) {
	dir := cfg.LocalDir
	if dir == "" {
		dir = "./data/files"
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve local storage dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create local storage dir: %w", err)
	}
	return &LocalStorageProvider{rootDir: abs, prefix: cfg.PathPrefix}, nil
}

// resolve 将存储 key 解析为本地绝对路径，带路径遍历防护。
//
// 前导 "/" 强制绝对化使 filepath.Clean 吃掉 ".."；随后再校验结果必须仍在 rootDir 之下，
// 双重防御 "../"、绝对路径、编码变体等绕过手段。raw key 以数字/字母开头，不会与 prefix 冲突。
func (s *LocalStorageProvider) resolve(key string) (string, error) {
	fullKey := applyStoragePrefix(s.prefix, key)
	cleaned := filepath.Clean("/" + fullKey)
	cleaned = strings.TrimPrefix(cleaned, "/")
	abs := filepath.Join(s.rootDir, cleaned)
	if !strings.HasPrefix(abs+string(os.PathSeparator), s.rootDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal detected: %s", key)
	}
	return abs, nil
}

// Upload 将文件写入本地磁盘。
func (s *LocalStorageProvider) Upload(ctx context.Context, reader io.Reader, key string, contentType string) (string, error) {
	abs, err := s.resolve(key)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}
	f, err := os.Create(abs)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return applyStoragePrefix(s.prefix, key), nil
}

// Download 打开本地文件供读取。调用方负责 Close 返回的 ReadCloser。
func (s *LocalStorageProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	abs, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

// Delete 删除本地文件。文件不存在视为成功（与 S3/OSS 语义对齐——FileService.Delete 依赖
// 此行为避免误报错而保留库行，使对象成为无法定位的孤儿）。
func (s *LocalStorageProvider) Delete(ctx context.Context, key string) error {
	abs, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

// PresignedURL 本地存储不支持预签名语义。
//
// 本 provider 的下载统一走应用层 serve 端点：FileService.GetDownloadURL 在 local 时直接
// 返回 serve URL（不调本方法）。返回 error 以防误用——若调用方未走 serve 链路却拿到伪造
// URL，反而制造难以排查的 404。AI 图片 re-host 等仍需真实预签名的链路，依赖
// acquireSyncImageFileSvc 的 IsStorageConfigured 守卫保持「要求对象存储」语义。
func (s *LocalStorageProvider) PresignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	return "", fmt.Errorf("local provider does not support presigned URLs")
}

// PresignedThumbnailURL 本地存储无服务端图片处理能力，回退到 PresignedURL（同 S3/MinIO/R2
// 的「返回原图」策略，不引入图片处理库）。local 模式下缩略图预览由前端 CSS 缩放。
func (s *LocalStorageProvider) PresignedThumbnailURL(ctx context.Context, key string, width int, expires time.Duration) (string, error) {
	return s.PresignedURL(ctx, key, expires)
}
