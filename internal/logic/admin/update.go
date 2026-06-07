package admin

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/logic/update"
)

// CheckUpdate checks GitHub Releases for a newer version
func (s *sAdmin) UpdateCheck(ctx context.Context, req *v1.UpdateCheckReq) (*v1.UpdateCheckRes, error) {
	result, err := update.CheckForUpdate(ctx, req.Force)
	if err != nil {
		return nil, gerror.NewCode(gcode.New(500, err.Error(), nil))
	}

	return &v1.UpdateCheckRes{
		CurrentVersion: result.CurrentVersion,
		LatestVersion:  result.LatestVersion,
		HasUpdate:      result.HasUpdate,
		ReleaseNotes:   result.ReleaseNotes,
		ReleaseURL:     result.ReleaseURL,
		PublishedAt:    result.PublishedAt,
		CheckedAt:      result.CheckedAt,
		DeploymentMode: result.DeploymentMode,
	}, nil
}

// GetUpdateStatus returns the current update status
func (s *sAdmin) UpdateStatus(ctx context.Context, req *v1.UpdateStatusReq) (*v1.UpdateStatusRes, error) {
	mgr := update.GetManager()
	status := mgr.GetStatus()

	res := &v1.UpdateStatusRes{
		CurrentVersion:    status.CurrentVersion,
		DeploymentMode:    status.DeploymentMode,
		Updating:          status.Updating,
		RollbackAvailable: status.RollbackAvailable,
		BackupVersion:     status.BackupVersion,
	}

	if status.LastCheck != nil {
		res.LastCheck = &v1.UpdateCheckRes{
			CurrentVersion: status.LastCheck.CurrentVersion,
			LatestVersion:  status.LastCheck.LatestVersion,
			HasUpdate:      status.LastCheck.HasUpdate,
			ReleaseNotes:   status.LastCheck.ReleaseNotes,
			ReleaseURL:     status.LastCheck.ReleaseURL,
			PublishedAt:    status.LastCheck.PublishedAt,
			CheckedAt:      status.LastCheck.CheckedAt,
			DeploymentMode: status.LastCheck.DeploymentMode,
		}
	}

	// Also check for cached check result
	if res.LastCheck == nil {
		cached := mgr.GetCheckResult()
		if cached != nil {
			res.LastCheck = &v1.UpdateCheckRes{
				CurrentVersion: cached.CurrentVersion,
				LatestVersion:  cached.LatestVersion,
				HasUpdate:      cached.HasUpdate,
				ReleaseNotes:   cached.ReleaseNotes,
				ReleaseURL:     cached.ReleaseURL,
				PublishedAt:    cached.PublishedAt,
				CheckedAt:      cached.CheckedAt,
				DeploymentMode: cached.DeploymentMode,
			}
		}
	}

	// Check rollback availability from disk
	if !res.RollbackAvailable {
		info, err := update.GetRollbackInfo()
		if err == nil && info != nil {
			res.RollbackAvailable = true
			res.BackupVersion = info.BackupVersion
		}
	}

	if status.Progress != nil {
		res.UpdateProgress = &v1.UpdateProgress{
			Phase:      status.Progress.Phase,
			Message:    status.Progress.Message,
			Percentage: status.Progress.Percentage,
			Error:      status.Progress.Error,
		}
	}

	return res, nil
}

// ExecuteUpdate triggers the system update
func (s *sAdmin) UpdateExecute(ctx context.Context, req *v1.UpdateExecuteReq) (*v1.UpdateExecuteRes, error) {
	// Docker mode check
	if update.IsDocker() {
		return nil, gerror.NewCode(gcode.New(consts.CodeUpdateNotSupported, consts.MsgUpdateNotSupported, nil))
	}

	// Check if already updating
	if update.GetManager().IsUpdating() {
		return nil, gerror.NewCode(gcode.New(consts.CodeUpdateAlreadyRunning, consts.MsgUpdateAlreadyRunning, nil))
	}

	// Get check result to find download URL
	cached := update.GetManager().GetCheckResult()
	if cached == nil {
		// Force a check first
		result, err := update.CheckForUpdate(ctx, true)
		if err != nil {
			return nil, gerror.New("检查更新失败: " + err.Error())
		}
		cached = result
	}

	if !cached.HasUpdate {
		return nil, gerror.NewCode(gcode.New(consts.CodeUpdateNotAvailable, consts.MsgUpdateNotAvailable, nil))
	}

	if cached.LatestVersion != req.Version {
		return nil, gerror.New(fmt.Sprintf("请求的版本 %s 与最新版本 %s 不匹配", req.Version, cached.LatestVersion))
	}

	if cached.DownloadURL == "" {
		return nil, gerror.NewCode(gcode.New(consts.CodeUpdateNotSupported, "当前平台没有可用的更新包", nil))
	}

	// Execute update
	if err := update.ExecuteUpdate(ctx, req.Version, cached.DownloadURL, cached.ChecksumURL, cached.AssetSize); err != nil {
		return nil, gerror.NewCode(gcode.New(consts.CodeUpdateDownloadFailed, err.Error(), nil))
	}

	return &v1.UpdateExecuteRes{
		Message: "系统更新已启动",
	}, nil
}

// RollbackUpdate rolls back to the previous version
func (s *sAdmin) UpdateRollback(ctx context.Context, req *v1.UpdateRollbackReq) (*v1.UpdateRollbackRes, error) {
	if update.IsDocker() {
		return nil, gerror.NewCode(gcode.New(consts.CodeUpdateNotSupported, consts.MsgUpdateNotSupported, nil))
	}

	if err := update.Rollback(ctx); err != nil {
		return nil, gerror.NewCode(gcode.New(consts.CodeUpdateRollbackFailed, consts.MsgUpdateRollbackFailed+": "+err.Error(), nil))
	}

	return &v1.UpdateRollbackRes{
		Message: "回滚已启动，系统将重启",
	}, nil
}
