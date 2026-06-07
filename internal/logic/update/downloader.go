package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	downloadTimeout = 5 * time.Minute
)

// DownloadResult holds the result of a download operation
type DownloadResult struct {
	FilePath string
	Size     int64
}

// DownloadFile downloads a file from the given URL to the update temp directory
// and reports progress via the manager
func DownloadFile(ctx context.Context, url, filename string, expectedSize int64) (*DownloadResult, error) {
	if url == "" {
		return nil, fmt.Errorf("download URL is empty")
	}

	// Ensure update directory exists
	_ = os.MkdirAll(updateDir, 0755)

	// Create temp file
	filePath := filepath.Join(updateDir, filename)
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Determine content size
	contentSize := expectedSize
	if contentSize == 0 {
		contentSize = resp.ContentLength
	}

	// Download with progress tracking
	written, err := downloadWithProgress(ctx, f, resp.Body, contentSize)
	if err != nil {
		_ = os.Remove(filePath)
		return nil, fmt.Errorf("download failed: %w", err)
	}

	g.Log().Infof(ctx, "Downloaded %s (%d bytes)", filename, written)

	return &DownloadResult{
		FilePath: filePath,
		Size:     written,
	}, nil
}

// downloadWithProgress copies from reader to writer and updates progress
func downloadWithProgress(ctx context.Context, dst *os.File, src io.Reader, totalSize int64) (int64, error) {
	buf := make([]byte, 32*1024) // 32KB buffer
	var written int64
	var lastReport time.Time

	for {
		nr, err := src.Read(buf)
		if nr > 0 {
			nw, err := dst.Write(buf[:nr])
			if err != nil {
				return written, err
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
			written += int64(nw)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return written, err
		}

		// Update progress every 500ms
		now := time.Now()
		if now.Sub(lastReport) > 500*time.Millisecond {
			lastReport = now
			percentage := 0
			if totalSize > 0 {
				percentage = int(written * 100 / totalSize)
				if percentage > 99 {
					percentage = 99
				}
			}
			sizeMB := float64(written) / 1024 / 1024
			manager.setProgress(PhaseDownloading,
				fmt.Sprintf("正在下载更新文件 (%.1f MB)...", sizeMB),
				percentage,
			)
		}
	}

	return written, nil
}
