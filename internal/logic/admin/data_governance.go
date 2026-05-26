package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

func (s *sAdmin) DataGovernanceSettingsGet(ctx context.Context, _ *v1.DataGovernanceSettingsGetReq) (*v1.DataGovernanceSettingsGetRes, error) {
	items := common.Config().GetCategoryWithValues(ctx, "data_governance")
	result := make(map[string]any, len(items))
	for _, item := range items {
		result[item.Key] = item.Value
	}
	return &v1.DataGovernanceSettingsGetRes{Data: result}, nil
}

// UpdateDataGovernanceSettings 更新数据治理设置
func (s *sAdmin) DataGovernanceSettingsUpdate(ctx context.Context, req *v1.DataGovernanceSettingsUpdateReq) (*v1.DataGovernanceSettingsUpdateRes, error) {
	for key, value := range req.Settings {
		if !common.IsRegisteredKey(key) {
			return nil, gerror.NewCodef(gcode.New(consts.CodeBadRequest, consts.MsgBadRequest, nil),
				"未知配置项: %s", key)
		}
		if err := common.Config().SetOption(ctx, key, value); err != nil {
			return nil, err
		}
	}
	return &v1.DataGovernanceSettingsUpdateRes{}, nil
}

// RequestDataExport 请求数据导出
func (s *sAdmin) DataGovernanceExport(ctx context.Context, req *v1.DataGovernanceExportReq) (*v1.DataGovernanceExportRes, error) {
	count, err := dao.TskTasks.Ctx(ctx).
		Where("handler", "data_export").
		Where("status", "pending").
		Where("payload->>'tenant_id'", fmt.Sprintf("%d", req.TenantID)).
		Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, gerror.NewCode(gcode.New(consts.CodeExportAlreadyPending, consts.MsgExportAlreadyPending, nil),
			consts.MsgExportAlreadyPending)
	}

	payload, _ := json.Marshal(map[string]any{
		"tenant_id":    req.TenantID,
		"scopes":       req.Scopes,
		"requested_by": common.GetCtxUserID(ctx),
	})
	result, err := dao.TskTasks.Ctx(ctx).Data(do.TskTasks{
		Name:       fmt.Sprintf("数据导出 [租户%d]", req.TenantID),
		Handler:    "data_export",
		Payload:    payload,
		Status:     "pending",
		MaxRetries: 1,
	}).Insert()
	if err != nil {
		return nil, err
	}
	taskID, _ := result.LastInsertId()
	return &v1.DataGovernanceExportRes{TaskID: taskID}, nil
}

// RequestDataDeletion 请求数据删除
func (s *sAdmin) DataGovernanceDeletion(ctx context.Context, req *v1.DataGovernanceDeletionReq) (*v1.DataGovernanceDeletionRes, error) {
	count, err := dao.TskTasks.Ctx(ctx).
		Where("handler", "data_deletion_request").
		Where("status IN (?)", []string{"pending", "running"}).
		Where("payload->>'tenant_id'", fmt.Sprintf("%d", req.TenantID)).
		Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, gerror.NewCode(gcode.New(consts.CodeDeletionRequestExists, consts.MsgDeletionRequestExists, nil),
			consts.MsgDeletionRequestExists)
	}

	payload, _ := json.Marshal(map[string]any{
		"tenant_id":    req.TenantID,
		"reason":       req.Reason,
		"requested_by": common.GetCtxUserID(ctx),
	})
	result, err := dao.TskTasks.Ctx(ctx).Data(do.TskTasks{
		Name:       fmt.Sprintf("数据删除 [租户%d]", req.TenantID),
		Handler:    "data_deletion_request",
		Payload:    payload,
		Status:     "pending",
		MaxRetries: 0,
	}).Insert()
	if err != nil {
		return nil, err
	}
	taskID, _ := result.LastInsertId()
	return &v1.DataGovernanceDeletionRes{TaskID: taskID}, nil
}

// TriggerDataCleanup 手动触发数据清理
func (s *sAdmin) DataGovernanceCleanup(ctx context.Context, _ *v1.DataGovernanceCleanupReq) (*v1.DataGovernanceCleanupRes, error) {
	if err := CleanupExpiredData(ctx); err != nil {
		return nil, err
	}
	return &v1.DataGovernanceCleanupRes{Message: "数据清理已触发"}, nil
}

// CleanupExpiredData 清理过期数据（定时任务入口）
func CleanupExpiredData(ctx context.Context) error {
	apiLogsDays := common.Config().GetInt(ctx, "data_retention_api_logs_days")
	opLogsDays := common.Config().GetInt(ctx, "data_retention_operation_logs_days")
	tempDays := common.Config().GetInt(ctx, "data_retention_temp_data_days")

	if apiLogsDays > 0 {
		if err := cleanupTableByDate(ctx, "aud_request_logs", "created_at", apiLogsDays); err != nil {
			g.Log().Errorf(ctx, "cleanup aud_request_logs: %v", err)
		}
	}
	if opLogsDays > 0 {
		if err := cleanupTableByDate(ctx, "aud_operation_logs", "created_at", opLogsDays); err != nil {
			g.Log().Errorf(ctx, "cleanup aud_operation_logs: %v", err)
		}
	}
	if tempDays > 0 {
		if err := cleanupTableByDate(ctx, "aud_sensitive_access_logs", "created_at", tempDays); err != nil {
			g.Log().Errorf(ctx, "cleanup aud_sensitive_access_logs: %v", err)
		}
	}
	if err := cleanupDeactivatedTenants(ctx); err != nil {
		g.Log().Errorf(ctx, "cleanup deactivated tenants: %v", err)
	}
	return nil
}

// CleanupExpiredExportFiles 清理过期导出文件
func CleanupExpiredExportFiles(ctx context.Context) error {
	expiryDays := common.Config().GetInt(ctx, "data_export_expiry_days")
	if expiryDays <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -expiryDays)

	var expiredFiles []struct {
		ID          int64  `json:"id"`
		StoragePath string `json:"storage_path"`
	}
	err := dao.FilFiles.Ctx(ctx).
		Where("created_at < ?", cutoff).
		Where("storage_path LIKE ?", "exports/%").
		Fields("id, storage_path").
		Scan(&expiredFiles)
	if err != nil {
		return err
	}
	for _, f := range expiredFiles {
		_, _ = dao.FilFiles.Ctx(ctx).Where("id", f.ID).Delete()
	}
	return nil
}

// CheckFileRetention 检查文件保留期
func CheckFileRetention(ctx context.Context) error {
	if !common.Config().GetBool(ctx, "file_retention_enabled") {
		return nil
	}
	return CleanupExpiredExportFiles(ctx)
}

func cleanupTableByDate(ctx context.Context, table, dateColumn string, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	batchSize := 5000
	for {
		result, err := g.DB().Ctx(ctx).Exec(ctx,
			fmt.Sprintf("DELETE FROM %s WHERE id IN (SELECT id FROM %s WHERE %s < $1 LIMIT %d)", table, table, dateColumn, batchSize), cutoff)
		if err != nil {
			return err
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			break
		}
		g.Log().Infof(ctx, "cleaned %d rows from %s (older than %d days)", rowsAffected, table, days)
	}
	return nil
}

func cleanupDeactivatedTenants(ctx context.Context) error {
	var tenants []struct {
		ID         int64  `json:"id"`
		StatusCode string `json:"status"`
	}
	err := dao.TntTenants.Ctx(ctx).
		WhereIn("status", []string{"frozen", "terminated", "closed"}).
		Where("data_removal_at < ?", gtime.Now()).
		Where("data_removal_at IS NOT NULL").
		Fields("id, status").
		Scan(&tenants)
	if err != nil {
		return err
	}
	for _, t := range tenants {
		g.Log().Infof(ctx, "cleaning data for deactivated tenant %d (status=%s)", t.ID, t.StatusCode)
		_, _ = dao.TntUsers.Ctx(ctx).Where("tenant_id", t.ID).
			Data(do.TntUsers{DisplayName: "[deleted]", Email: fmt.Sprintf("deleted_%d@deleted.local", t.ID)}).Update()
		_, _ = dao.ApiKeys.Ctx(ctx).Where("tenant_id", t.ID).Delete()
		_, _ = dao.TntTenants.Ctx(ctx).Where("id", t.ID).
			Data(do.TntTenants{DataRemovalAt: gtime.Now()}).Update()
	}
	return nil
}
