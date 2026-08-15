package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/consts"
)

const (
	// updateDir is the temp directory for update operations
	updateDir = "/tmp/team-api-update"
	// rollbackFile stores rollback metadata
	rollbackFile = "rollback.json"
	// pendingVerificationFile marks a just-updated process
	pendingVerificationFile = "pending_verification"
	// backupMaxAge is how long old backups are kept
	backupMaxAge = 7 * 24 * time.Hour
	// progressExpire 终态进度（complete/failed）的保留时长：
	// 覆盖一次升级完成后管理员轮询确认的窗口，超时即不再下发
	progressExpire = 30 * time.Minute
)

// manager is the singleton UpdateManager
var manager *UpdateManager

func init() {
	manager = &UpdateManager{}
	manager.updating.Store(false)
}

// GetManager returns the singleton UpdateManager
func GetManager() *UpdateManager {
	return manager
}

// InitManager initializes the update manager and cleans old backups
func InitManager(ctx context.Context) {
	// Ensure update directory exists
	_ = os.MkdirAll(updateDir, 0755)

	// Clean old backups
	cleanOldBackups(ctx)

	g.Log().Info(ctx, "Update manager initialized")
}

// selfHealthURL 构造升级后自检用的本机健康检查地址。
// 端口优先取 ghttp 运行时实际监听端口（GetListenedPort），服务尚未完成监听时
// 从配置 server.address 兜底解析，避免部署自定义端口后仍探测默认的 18888。
// 始终探测 127.0.0.1（GetListenedAddress 可能是 0.0.0.0/[::] 这类通配地址，
// 不能直接作为连接目标）。
func selfHealthURL() string {
	if port := g.Server().GetListenedPort(); port > 0 {
		return fmt.Sprintf("http://127.0.0.1:%d/api/health", port)
	}

	// 兜底：服务未开始监听（理论上自检在启动 5 秒后才开始，很难走到这里）
	if addr := g.Cfg().MustGet(context.Background(), "server.address").String(); addr != "" {
		if _, portStr, err := net.SplitHostPort(addr); err == nil {
			if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
				return fmt.Sprintf("http://127.0.0.1:%d/api/health", port)
			}
		}
	}

	// 最终兜底：默认端口
	return "http://127.0.0.1:18888/api/health"
}

// CheckPendingVerification checks if this process just started after an update
// and verifies the new version is healthy
func CheckPendingVerification(ctx context.Context) {
	pendingPath := filepath.Join(updateDir, pendingVerificationFile)
	data, err := os.ReadFile(pendingPath)
	if err != nil {
		return // no pending verification
	}

	var info struct {
		Version    string `json:"version"`
		OldBinary  string `json:"old_binary"`
		OldVersion string `json:"old_version"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		_ = os.Remove(pendingPath)
		return
	}

	g.Log().Infof(ctx, "Detected pending update verification for version %s", info.Version)

	// Verify health in background
	go func() {
		time.Sleep(5 * time.Second)

		// 回滚入口展示用的版本号：优先取 pending 标记里的旧版本号；
		// 旧版升级流程写的标记没有该字段，回落读 rollback.json
		backupVersion := info.OldVersion
		if backupVersion == "" {
			if rb, rbErr := loadRollbackInfo(); rbErr == nil && rb != nil {
				backupVersion = rb.BackupVersion
			}
		}

		verifiedAt := time.Now()
		// Check own health endpoint（端口取运行时实际监听端口，见 selfHealthURL 注释）
		client := g.Client()
		resp, err := client.Get(ctx, selfHealthURL())
		if err != nil || resp == nil {
			g.Log().Warningf(ctx, "Update verification health check failed: %v", err)
			// Wait and retry
			time.Sleep(10 * time.Second)
			resp, err = client.Get(ctx, selfHealthURL())
		}

		if err == nil && resp != nil {
			defer resp.Close()
			if resp.StatusCode == 200 {
				g.Log().Infof(ctx, "Update to %s verified successfully", info.Version)
				_ = os.Remove(pendingPath)

				// Update status in Redis
				status := &Status{
					CurrentVersion:    consts.Version,
					DeploymentMode:    GetDeploymentMode(),
					Updating:          false,
					Progress:          &Progress{Phase: PhaseComplete, Message: "更新完成", Percentage: 100, FinishedAt: &verifiedAt},
					RollbackAvailable: info.OldBinary != "",
					BackupVersion:     backupVersion,
				}
				manager.status.Store(status)

				// Clean up .old file
				if info.OldBinary != "" {
					_ = os.Remove(info.OldBinary + ".old")
				}
				return
			}
		}

		// Health check failed after retries
		g.Log().Errorf(ctx, "Update to %s verification failed - health check not passing", info.Version)
		status := &Status{
			CurrentVersion: consts.Version,
			DeploymentMode: GetDeploymentMode(),
			Updating:       false,
			Progress: &Progress{
				Phase:      PhaseFailed,
				Message:    "更新后健康检查失败，建议回滚",
				Error:      "health check failed after update",
				FinishedAt: &verifiedAt,
			},
			RollbackAvailable: info.OldBinary != "",
			BackupVersion:     backupVersion,
		}
		manager.status.Store(status)
	}()
}

// BackgroundCheck performs an update check (called by cron)
func BackgroundCheck(ctx context.Context) error {
	result, err := CheckForUpdate(ctx, false)
	if err != nil {
		g.Log().Warningf(ctx, "Background update check failed: %v", err)
		return err
	}

	if result != nil && result.HasUpdate {
		g.Log().Infof(ctx, "New version available: %s (current: %s)", result.LatestVersion, result.CurrentVersion)
	}

	return nil
}

// GetStatus returns the current update status.
// 终态进度（complete/failed）超过 progressExpire 后视为过期，返回时剔除，
// 避免上次升级留下的快照在进程内永久残留（快照本身保留在内存中不影响，
// 下一次 setProgress 会以无进度的状态重建）
func (m *UpdateManager) GetStatus() *Status {
	val := m.status.Load()
	if val == nil {
		return &Status{
			CurrentVersion: consts.Version,
			DeploymentMode: GetDeploymentMode(),
			Updating:       false,
		}
	}
	status := val.(*Status)

	if p := status.Progress; p != nil && isTerminalPhase(p.Phase) &&
		p.FinishedAt != nil && time.Since(*p.FinishedAt) > progressExpire {
		filtered := *status
		filtered.Progress = nil
		return &filtered
	}
	return status
}

// GetCheckResult returns the cached check result
func (m *UpdateManager) GetCheckResult() *CheckResult {
	val := m.checkResult.Load()
	if val == nil {
		return nil
	}
	return val.(*CheckResult)
}

// IsUpdating returns whether an update is in progress
func (m *UpdateManager) IsUpdating() bool {
	return m.updating.Load()
}

// setProgress updates the current progress
func (m *UpdateManager) setProgress(phase, message string, percentage int) {
	status := m.GetStatus()
	status.Updating = true
	status.Progress = &Progress{
		Phase:      phase,
		Message:    message,
		Percentage: percentage,
	}
	m.status.Store(status)
}

// setProgressError marks the update as failed
func (m *UpdateManager) setProgressError(message, errMsg string) {
	status := m.GetStatus()
	status.Updating = false
	failedAt := time.Now()
	status.Progress = &Progress{
		Phase:      PhaseFailed,
		Message:    message,
		Error:      errMsg,
		FinishedAt: &failedAt,
	}
	m.status.Store(status)
	m.updating.Store(false)
}

// cleanOldBackups removes backup files older than backupMaxAge
func cleanOldBackups(ctx context.Context) {
	exe, err := os.Executable()
	if err != nil {
		return
	}

	dir := filepath.Dir(exe)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	base := filepath.Base(exe)
	now := time.Now()

	for _, entry := range entries {
		name := entry.Name()
		// Match pattern: {binary}.backup.{timestamp} or {binary}.old
		if len(name) > len(base) && name[:len(base)] == base {
			suffix := name[len(base):]
			if len(suffix) > 8 && suffix[:8] == ".backup." {
				info, err := entry.Info()
				if err != nil {
					continue
				}
				if now.Sub(info.ModTime()) > backupMaxAge {
					_ = os.Remove(filepath.Join(dir, name))
					g.Log().Debugf(ctx, "Cleaned old backup: %s", name)
				}
			}
		}
	}
}

// GetDeploymentMode returns the current deployment mode
func GetDeploymentMode() string {
	if IsDocker() {
		return DeploymentDocker
	}
	return DeploymentBinary
}

// getPlatformAssetName constructs the expected asset name for the current platform
func getPlatformAssetName(version string) string {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	ext := "tar.gz"
	if osName == "windows" {
		ext = "zip"
	}
	// Strip 'v' prefix if present
	v := version
	if len(v) > 0 && v[0] == 'v' {
		v = v[1:]
	}
	return fmt.Sprintf("team-api-%s-%s-%s.%s", v, osName, arch, ext)
}
