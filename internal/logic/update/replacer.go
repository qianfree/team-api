package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/consts"
)

// executablePath 获取当前二进制路径。默认 os.Executable；本地集成测试可覆盖为
// 指向临时目录的假二进制，避免真实替换测试进程自身（见 update_integration_test.go）。
var executablePath = os.Executable

// ExecuteUpdate runs the full update process in a goroutine
func ExecuteUpdate(ctx context.Context, targetVersion, downloadURL, checksumURL string, assetSize int64) error {
	// Check if already updating
	if !manager.updating.CompareAndSwap(false, true) {
		return fmt.Errorf("update already in progress")
	}

	// Docker mode check
	if IsDocker() {
		manager.updating.Store(false)
		return fmt.Errorf("auto-update is not supported in Docker mode")
	}

	// Windows 不支持在线更新：无 exec 换壳语义（见 exec_windows.go），
	// 且 Windows 上运行中的二进制无法被重命名，替换必然失败，显式拒绝
	if runtime.GOOS == "windows" {
		manager.updating.Store(false)
		return fmt.Errorf("auto-update is not supported on Windows")
	}

	// Check download URL
	if downloadURL == "" {
		manager.updating.Store(false)
		return fmt.Errorf("no download URL available for this platform (%s/%s)", runtime.GOOS, runtime.GOARCH)
	}

	// Initialize status
	manager.status.Store(&Status{
		CurrentVersion: consts.Version,
		DeploymentMode: DeploymentBinary,
		Updating:       true,
		Progress:       &Progress{Phase: PhaseDownloading, Message: "正在准备下载...", Percentage: 0},
	})

	// 脱离 HTTP 请求 context：ExecuteUpdate 返回后 handler 随即结束，请求 ctx 会被取消，
	// 若复用该 ctx，后台下载/校验/替换任务会立即收到 context canceled。
	// 改用独立的 GoFrame context（保留 trace 用于日志关联），并加总超时兜底防止任务挂死。
	updateCtx, cancel := context.WithTimeout(gctx.New(), 15*time.Minute)
	go func() {
		defer cancel()
		performUpdate(updateCtx, targetVersion, downloadURL, checksumURL, assetSize)
	}()

	return nil
}

// performUpdate is the main update routine
func performUpdate(ctx context.Context, targetVersion, downloadURL, checksumURL string, assetSize int64) {
	defer func() {
		if r := recover(); r != nil {
			g.Log().Errorf(ctx, "Update panic: %v", r)
			manager.setProgressError("更新过程中发生异常", fmt.Sprintf("%v", r))
		}
	}()

	if _, err := stageUpdate(ctx, targetVersion, downloadURL, checksumURL, assetSize); err != nil {
		return // stageUpdate 已设置失败状态并记录日志
	}

	// 触发重启：SIGTERM 优雅关闭 → cmd.go defer 链排空任务池/flush 日志
	// → MaybeExecRestart 用 syscall.Exec 原地换壳为新版本（同 PID，不依赖进程管理器）
	manager.setProgress(PhaseRestarting, "正在重启服务...", 90)
	g.Log().Info(ctx, "Update complete, restarting...")
	gracefulExit(ctx, "update complete")
}

// stageUpdate 执行下载→校验→备份→替换→写标记的完整流水线，返回替换后的新二进制路径。
// 与 performUpdate 分离，便于本地集成测试直接驱动生产流水线而不触发进程重启（exec）。
func stageUpdate(ctx context.Context, targetVersion, downloadURL, checksumURL string, assetSize int64) (string, error) {
	// Step 1: Download
	assetName := getPlatformAssetName(targetVersion)
	dlResult, err := DownloadFile(ctx, downloadURL, assetName, assetSize)
	if err != nil {
		manager.setProgressError("下载更新文件失败", err.Error())
		g.Log().Errorf(ctx, "Download failed: %v", err)
		return "", err
	}

	// Step 2: Verify（校验失败一律中止，不放行未通过完整性校验的更新包）
	if err := VerifyChecksum(ctx, dlResult.FilePath, checksumURL, assetName); err != nil {
		manager.setProgressError("文件校验失败，更新包可能已破损", err.Error())
		g.Log().Errorf(ctx, "Checksum verification failed: %v", err)
		_ = os.Remove(dlResult.FilePath)
		return "", err
	}

	// Step 3: 定位当前二进制路径
	// 新二进制必须提取到与当前二进制相同的目录（同一文件系统），
	// 否则后续 os.Rename 会因跨设备（EXDEV）失败，如 /tmp 与部署目录不在同一分区
	currentExe, err := executablePath()
	if err != nil {
		manager.setProgressError("获取当前程序路径失败", err.Error())
		_ = os.Remove(dlResult.FilePath)
		return "", err
	}
	// Resolve symlinks
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		g.Log().Warningf(ctx, "Failed to resolve symlink: %v", err)
	}

	// Step 4: Extract binary from tarball（提取到当前二进制同目录）
	manager.setProgress(PhaseBackingUp, "正在提取二进制文件...", 10)
	newBinaryPath, err := extractBinary(ctx, dlResult.FilePath, currentExe+".new")
	if err != nil {
		manager.setProgressError("提取文件失败", err.Error())
		g.Log().Errorf(ctx, "Extract failed: %v", err)
		_ = os.Remove(dlResult.FilePath)
		return "", err
	}
	_ = os.Remove(dlResult.FilePath) // clean up tarball

	// Step 5: Backup current binary
	manager.setProgress(PhaseBackingUp, "正在备份当前版本...", 30)

	backupPath := fmt.Sprintf("%s.backup.%s", currentExe, gtime.Now().Format("YmdHis"))
	if err := copyFile(currentExe, backupPath); err != nil {
		manager.setProgressError("备份当前版本失败", err.Error())
		_ = os.Remove(newBinaryPath)
		return "", err
	}
	g.Log().Infof(ctx, "Backed up current binary to %s", backupPath)

	// Save rollback info
	rollbackInfo := &RollbackInfo{
		BackupPath:    backupPath,
		OriginalPath:  currentExe,
		BackupVersion: consts.Version,
		Timestamp:     gtime.Now().Format("Y-m-d H:i:s"),
	}
	if err := saveRollbackInfo(rollbackInfo); err != nil {
		g.Log().Warningf(ctx, "Failed to save rollback info: %v", err)
	}

	// Step 6: Replace binary
	manager.setProgress(PhaseReplacing, "正在替换程序文件...", 60)
	if err := replaceBinary(ctx, currentExe, newBinaryPath); err != nil {
		manager.setProgressError("替换程序文件失败", err.Error())
		g.Log().Errorf(ctx, "Replace failed: %v", err)
		_ = os.Remove(newBinaryPath)
		return "", err
	}

	// Step 7: Write pending verification marker
	pendingData, _ := json.Marshal(struct {
		Version    string `json:"version"`
		OldBinary  string `json:"old_binary"`
		OldVersion string `json:"old_version"`
	}{
		Version: targetVersion,
		// 旧版本被 replaceBinary 重命名为 {exe}.old 的实际路径：自检成功后据此清理；
		// 回滚仍走 rollback.json 的 .backup.* 副本，不受该清理影响
		OldBinary: currentExe + ".old",
		// 记录升级前版本号：自检完成后展示 rollback 入口时需要显示版本而非备份路径
		OldVersion: consts.Version,
	})
	_ = os.WriteFile(filepath.Join(updateDir, pendingVerificationFile), pendingData, 0644)

	// 记录待重启的新版本路径：cmd.go 退出链末尾用 syscall.Exec 原地换壳为新版本，
	// 同 PID 继续运行，不依赖外部进程管理器拉起（见 gracefulExit 注释）
	manager.SetRestartBinary(currentExe)

	return currentExe, nil
}

// extractBinary extracts the team-api binary from a tar.gz archive to outputPath.
// outputPath 必须与被替换的二进制位于同一文件系统（通常为同目录），
// 否则 replaceBinary 中的 os.Rename 会因跨设备（EXDEV）失败
func extractBinary(ctx context.Context, tarballPath, outputPath string) (string, error) {
	f, err := os.Open(tarballPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gzip open failed: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	// Look for the binary inside the tarball
	// Expected structure: team-api-{version}-{os}-{arch}/team-api
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar read failed: %w", err)
		}

		// Look for the binary file (not directory)
		baseName := filepath.Base(hdr.Name)
		if baseName == "team-api" && hdr.Typeflag == tar.TypeReg {
			outf, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return "", err
			}
			defer outf.Close()

			if _, err := io.Copy(outf, tr); err != nil {
				_ = os.Remove(outputPath)
				return "", fmt.Errorf("extract failed: %w", err)
			}

			return outputPath, nil
		}
	}

	return "", fmt.Errorf("binary not found in tarball")
}

// replaceBinary replaces the current binary with the new one using the rename strategy
func replaceBinary(ctx context.Context, currentExe, newBinary string) error {
	// Strategy:
	// 1. rename current → current.old (safe because Linux allows renaming running binary)
	// 2. rename new → current
	// 3. chmod new binary
	oldPath := currentExe + ".old"

	// Remove any leftover .old file
	_ = os.Remove(oldPath)

	// Rename current binary to .old
	if err := os.Rename(currentExe, oldPath); err != nil {
		return fmt.Errorf("rename current to .old failed: %w", err)
	}

	// Rename new binary to current path
	if err := os.Rename(newBinary, currentExe); err != nil {
		// Attempt rollback: rename .old back
		_ = os.Rename(oldPath, currentExe)
		return fmt.Errorf("rename new to current failed: %w", err)
	}

	// Ensure executable permission
	if err := os.Chmod(currentExe, 0755); err != nil {
		g.Log().Warningf(ctx, "Failed to chmod binary: %v", err)
	}

	g.Log().Infof(ctx, "Binary replaced: %s -> %s", oldPath, currentExe)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Get source file permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		_ = os.Remove(dst)
		return err
	}

	return nil
}

// saveRollbackInfo persists rollback metadata to disk
func saveRollbackInfo(info *RollbackInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(updateDir, rollbackFile), data, 0644)
}
