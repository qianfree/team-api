package update

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/consts"
)

// Rollback rolls back to the previous version
func Rollback(ctx context.Context) error {
	if IsDocker() {
		return fmt.Errorf("rollback is not supported in Docker mode")
	}

	if manager.IsUpdating() {
		return fmt.Errorf("cannot rollback while update is in progress")
	}

	// Read rollback info
	info, err := loadRollbackInfo()
	if err != nil {
		return fmt.Errorf("no rollback information available: %w", err)
	}

	// Verify backup file exists
	if _, err := os.Stat(info.BackupPath); err != nil {
		return fmt.Errorf("backup file not found: %s", info.BackupPath)
	}

	g.Log().Infof(ctx, "Rolling back from %s to %s using backup %s",
		consts.Version, info.BackupVersion, info.BackupPath)

	// Current executable path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		g.Log().Warningf(ctx, "Failed to resolve symlink: %v", err)
	}

	// Replace current binary with backup
	oldPath := currentExe + ".old"
	_ = os.Remove(oldPath)

	if err := os.Rename(currentExe, oldPath); err != nil {
		return fmt.Errorf("failed to rename current binary: %w", err)
	}

	if err := os.Rename(info.BackupPath, currentExe); err != nil {
		// Try to restore
		_ = os.Rename(oldPath, currentExe)
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	_ = os.Chmod(currentExe, 0755)

	// Clean up
	_ = os.Remove(filepath.Join(updateDir, rollbackFile))
	_ = os.Remove(filepath.Join(updateDir, pendingVerificationFile))
	_ = os.Remove(oldPath)

	g.Log().Infof(ctx, "Rollback to %s complete, exiting for restart...", info.BackupVersion)

	// Exit — process supervisor will restart with the old binary
	os.Exit(0)
	return nil
}

// loadRollbackInfo reads rollback metadata from disk
func loadRollbackInfo() (*RollbackInfo, error) {
	data, err := os.ReadFile(filepath.Join(updateDir, rollbackFile))
	if err != nil {
		return nil, err
	}

	var info RollbackInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// GetRollbackInfo returns the current rollback information (if any)
func GetRollbackInfo() (*RollbackInfo, error) {
	return loadRollbackInfo()
}
