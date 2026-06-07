package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// VerifyChecksum verifies the SHA256 checksum of the downloaded file
func VerifyChecksum(ctx context.Context, filePath, checksumURL, expectedFilename string) error {
	manager.setProgress(PhaseVerifying, "正在校验文件完整性...", 50)

	// Compute SHA256 of the downloaded file
	actualHash, err := computeSHA256(ctx, filePath)
	if err != nil {
		return fmt.Errorf("failed to compute SHA256: %w", err)
	}

	g.Log().Debugf(ctx, "Computed SHA256: %s", actualHash)

	// If no checksum URL, skip verification (log warning)
	if checksumURL == "" {
		g.Log().Warning(ctx, "No checksum URL available, skipping verification")
		return nil
	}

	// Download checksums file
	expectedHash, err := fetchExpectedHash(ctx, checksumURL, expectedFilename)
	if err != nil {
		g.Log().Warningf(ctx, "Failed to fetch checksum file, skipping verification: %v", err)
		return nil
	}

	manager.setProgress(PhaseVerifying, "正在对比校验值...", 80)

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	manager.setProgress(PhaseVerifying, "文件校验通过", 100)
	g.Log().Info(ctx, "File checksum verified successfully")

	return nil
}

// computeSHA256 computes the SHA256 hash of a file with progress reporting
func computeSHA256(ctx context.Context, filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	hash := sha256.New()
	buf := make([]byte, 64*1024) // 64KB buffer
	var processed int64
	var lastReport time.Time

	for {
		nr, err := f.Read(buf)
		if nr > 0 {
			hash.Write(buf[:nr])
			processed += int64(nr)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// Update progress
		now := time.Now()
		if now.Sub(lastReport) > 500*time.Millisecond {
			lastReport = now
			percentage := 30
			if info.Size() > 0 {
				percentage = int(processed * 30 / info.Size())
				if percentage > 29 {
					percentage = 29
				}
			}
			manager.setProgress(PhaseVerifying, "正在计算文件校验值...", percentage)
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// fetchExpectedHash downloads the checksums file and finds the hash for the target filename
func fetchExpectedHash(ctx context.Context, checksumURL, filename string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", checksumURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum download returned %d", resp.StatusCode)
	}

	// Parse checksums file format:
	// <hash>  <filename>
	// or
	// <hash> *<filename>
	scanner := bufio.NewScanner(resp.Body)
	baseFilename := filepath.Base(filename)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hash := parts[0]
			file := strings.TrimPrefix(parts[1], "*")
			if file == baseFilename || file == filename {
				return strings.ToLower(hash), nil
			}
		}
	}

	return "", fmt.Errorf("checksum for %s not found in checksums file", filename)
}
