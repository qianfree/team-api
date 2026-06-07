package update

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

// CheckPendingVerification checks if this process just started after an update
// and verifies the new version is healthy
func CheckPendingVerification(ctx context.Context) {
	pendingPath := filepath.Join(updateDir, pendingVerificationFile)
	data, err := os.ReadFile(pendingPath)
	if err != nil {
		return // no pending verification
	}

	var info struct {
		Version   string `json:"version"`
		OldBinary string `json:"old_binary"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		_ = os.Remove(pendingPath)
		return
	}

	g.Log().Infof(ctx, "Detected pending update verification for version %s", info.Version)

	// Verify health in background
	go func() {
		time.Sleep(5 * time.Second)

		// Check own health endpoint
		client := g.Client()
		resp, err := client.Get(ctx, "http://127.0.0.1:18888/api/health")
		if err != nil || resp == nil {
			g.Log().Warningf(ctx, "Update verification health check failed: %v", err)
			// Wait and retry
			time.Sleep(10 * time.Second)
			resp, err = client.Get(ctx, "http://127.0.0.1:18888/api/health")
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
					Progress:          &Progress{Phase: PhaseComplete, Message: "更新完成", Percentage: 100},
					RollbackAvailable: info.OldBinary != "",
					BackupVersion:     info.OldBinary,
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
				Phase:   PhaseFailed,
				Message: "更新后健康检查失败，建议回滚",
				Error:   "health check failed after update",
			},
			RollbackAvailable: info.OldBinary != "",
			BackupVersion:     info.OldBinary,
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

// GetStatus returns the current update status
func (m *UpdateManager) GetStatus() *Status {
	val := m.status.Load()
	if val == nil {
		return &Status{
			CurrentVersion: consts.Version,
			DeploymentMode: GetDeploymentMode(),
			Updating:       false,
		}
	}
	return val.(*Status)
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
	status.Progress = &Progress{
		Phase:   PhaseFailed,
		Message: message,
		Error:   errMsg,
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
