package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
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
		Where("(payload->>'tenant_id')::bigint = ?", req.TenantID).
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
		Where("(payload->>'tenant_id')::bigint = ?", req.TenantID).
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

// FileCleanupResult 报告一次文件保留期清理删除的文件数量。
type FileCleanupResult struct {
	ExportsDeleted int `json:"exports_deleted"`
	ImagesDeleted  int `json:"images_deleted"`
}

// deleteExpiredFiles 删除 created_at 早于 cutoff 的 fil_files 行及其存储对象。
// exports=true 匹配 storage_path LIKE 'exports/%'（导出文件）；exports=false 匹配
// 其余（AI re-host 图片等 provider 上传的文件）。删除经 FileService.Delete 完成，
// 会一并删除桶中对象（而非仅删库行，避免存储泄漏）。fileSvc 为 nil 时（对象存储未
// 配置）退化为仅删库行。分批处理以限制内存与单次事务规模。
func deleteExpiredFiles(ctx context.Context, fileSvc *common.FileService, cutoff time.Time, exports bool) (int, error) {
	const batchSize = 500
	total := 0
	for {
		m := dao.FilFiles.Ctx(ctx).Where("created_at < ?", cutoff)
		if exports {
			m = m.Where("storage_path LIKE ?", "exports/%")
		} else {
			m = m.Where("storage_path NOT LIKE ?", "exports/%")
		}

		var batch []struct {
			ID int64 `json:"id"`
		}
		if err := m.Fields("id").OrderAsc("id").Limit(batchSize).Scan(&batch); err != nil {
			return total, err
		}
		if len(batch) == 0 {
			break
		}

		deletedInBatch := 0
		for _, f := range batch {
			if fileSvc != nil {
				// FileService.Delete 删对象(失败仅告警)后删行；返回硬错误表示行未删。
				if err := fileSvc.Delete(ctx, f.ID); err != nil {
					g.Log().Warningf(ctx, "file retention: delete file %d failed: %v", f.ID, err)
					continue
				}
			} else if _, err := dao.FilFiles.Ctx(ctx).Where("id", f.ID).Delete(); err != nil {
				g.Log().Warningf(ctx, "file retention: delete row %d failed: %v", f.ID, err)
				continue
			}
			total++
			deletedInBatch++
		}

		// 整批未能删除任何一行（删除持续失败）——停止，避免同一批被反复重查形成热循环。
		if deletedInBatch == 0 {
			g.Log().Warningf(ctx, "file retention: no progress in batch, stopping (check storage/db)")
			break
		}
		if len(batch) < batchSize {
			break
		}
	}
	if total > 0 {
		g.Log().Infof(ctx, "file retention: removed %d files (exports=%v, before %s)", total, exports, cutoff.Format("2006-01-02"))
	}
	return total, nil
}

// buildFileService 从数据库配置构造 FileService；对象存储未配置时返回 nil（清理退化为仅删库行）。
func buildFileService(ctx context.Context) *common.FileService {
	fileSvc, err := common.NewFileServiceFromConfig(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "file retention: object storage unavailable, pruning db rows only: %v", err)
		return nil
	}
	return fileSvc
}

// CleanupExpiredExportFiles 清理过期导出文件（含存储对象）。由 export_file_cleanup cron 调用。
func CleanupExpiredExportFiles(ctx context.Context) error {
	expiryDays := common.Config().GetInt(ctx, "data_export_expiry_days")
	if expiryDays <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -expiryDays)
	_, err := deleteExpiredFiles(ctx, buildFileService(ctx), cutoff, true)
	return err
}

// CleanupExpiredImages 清理过期的 AI re-host 图片及其它 provider 上传文件（含存储对象）。
// file_image_retention_days<=0 表示关闭（默认），直接跳过。
func CleanupExpiredImages(ctx context.Context) (int, error) {
	retentionDays := common.Config().GetInt(ctx, "file_image_retention_days")
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	return deleteExpiredFiles(ctx, buildFileService(ctx), cutoff, false)
}

// CheckFileRetention 由 file_retention_check cron 调用：受 file_retention_enabled 开关控制，
// 清理过期图片文件（导出文件由独立的 export_file_cleanup cron 负责）。
func CheckFileRetention(ctx context.Context) error {
	if !common.Config().GetBool(ctx, "file_retention_enabled") {
		return nil
	}
	_, err := CleanupExpiredImages(ctx)
	return err
}

// RunFileRetentionNow 立即执行一次完整保留期清理（导出 + 图片），返回各自删除数量。
// 供管理后台「手动触发清理」端点调用；不受 file_retention_enabled 开关限制。
func RunFileRetentionNow(ctx context.Context) (*FileCleanupResult, error) {
	res := &FileCleanupResult{}
	fileSvc := buildFileService(ctx)

	if days := common.Config().GetInt(ctx, "data_export_expiry_days"); days > 0 {
		n, err := deleteExpiredFiles(ctx, fileSvc, time.Now().AddDate(0, 0, -days), true)
		if err != nil {
			return res, err
		}
		res.ExportsDeleted = n
	}
	if days := common.Config().GetInt(ctx, "file_image_retention_days"); days > 0 {
		n, err := deleteExpiredFiles(ctx, fileSvc, time.Now().AddDate(0, 0, -days), false)
		if err != nil {
			return res, err
		}
		res.ImagesDeleted = n
	}
	return res, nil
}

func cleanupTableByDate(ctx context.Context, table, dateColumn string, days int) error {
	allowedCleanupTables := map[string]map[string]bool{
		"aud_request_logs":          {"created_at": true},
		"aud_operation_logs":        {"created_at": true},
		"aud_sensitive_access_logs": {"created_at": true},
	}
	cols, ok := allowedCleanupTables[table]
	if !ok || !cols[dateColumn] {
		return fmt.Errorf("invalid table/column for cleanup: %s.%s", table, dateColumn)
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	batchSize := 5000

	// Build safe DELETE SQL via switch/case to avoid fmt.Sprintf on table/column names.
	// The whitelist above guarantees table and dateColumn are from a trusted set.
	var deleteSQL string
	switch table {
	case "aud_request_logs":
		deleteSQL = "DELETE FROM aud_request_logs WHERE id IN (SELECT id FROM aud_request_logs WHERE created_at < ? LIMIT ?)"
	case "aud_operation_logs":
		deleteSQL = "DELETE FROM aud_operation_logs WHERE id IN (SELECT id FROM aud_operation_logs WHERE created_at < ? LIMIT ?)"
	case "aud_sensitive_access_logs":
		deleteSQL = "DELETE FROM aud_sensitive_access_logs WHERE id IN (SELECT id FROM aud_sensitive_access_logs WHERE created_at < ? LIMIT ?)"
	default:
		return fmt.Errorf("invalid table for cleanup: %s", table)
	}

	// aud_request_logs 走独立库，其余审计表走主库
	db := g.DB()
	if table == "aud_request_logs" {
		db = common.GetAuditDB()
	}

	for {
		result, err := db.Ctx(ctx).Exec(ctx, deleteSQL, cutoff, batchSize)
		if err != nil {
			return err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
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

		// Use transaction to ensure atomicity: if any step fails, the tenant's data
		// remains consistent (no partial anonymization/deletion).
		err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			tenantID := t.ID
			if _, err := tx.Model("tnt_users").Ctx(ctx).Where("tenant_id", tenantID).
				Data(do.TntUsers{DisplayName: "[deleted]", Email: fmt.Sprintf("deleted_%d@deleted.local", tenantID)}).Update(); err != nil {
				return gerror.Wrapf(err, "anonymize users for tenant %d", tenantID)
			}
			if _, err := tx.Model("api_keys").Ctx(ctx).Where("tenant_id", tenantID).Delete(); err != nil {
				return gerror.Wrapf(err, "delete api keys for tenant %d", tenantID)
			}
			if _, err := tx.Model("tnt_tenants").Ctx(ctx).Where("id", tenantID).
				Data(do.TntTenants{DataRemovalAt: gtime.Now()}).Update(); err != nil {
				return gerror.Wrapf(err, "update data removal for tenant %d", tenantID)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
