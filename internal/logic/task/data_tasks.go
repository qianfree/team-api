package task

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

type ExportPayload struct {
	TenantID    int64    `json:"tenant_id"`
	Scopes      []string `json:"scopes"`
	RequestedBy int64    `json:"requested_by"`
}

type DeletionPayload struct {
	TenantID    int64  `json:"tenant_id"`
	Reason      string `json:"reason"`
	RequestedBy int64  `json:"requested_by"`
}

func init() {
	RegisterHandler("data_export", handleDataExport)
	RegisterHandler("data_deletion_request", handleDeletionRequest)
	RegisterHandler("data_export_cleanup", handleExportCleanup)
}

func handleDataExport(ctx context.Context, payload json.RawMessage) (any, error) {
	var p ExportPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}

	exportData := make(map[string]any)
	for _, scope := range p.Scopes {
		switch scope {
		case "members":
			var members []map[string]any
			result, err := dao.TntUsers.Ctx(ctx).Where("tenant_id", p.TenantID).
				Fields("id, username, display_name, role, status, created_at").
				All()
			if err != nil {
				g.Log().Warningf(ctx, "export members for tenant %d: %v", p.TenantID, err)
			} else {
				for _, rec := range result {
					m := rec.Map()
					delete(m, "email")
					members = append(members, m)
				}
			}
			exportData["members"] = members
		case "usage":
			var logs []map[string]any
			result, err := dao.BilUsageLogs.Ctx(ctx).Where("tenant_id", p.TenantID).
				OrderDesc("created_at").Limit(10000).
				Fields("id, model_name, input_tokens, output_tokens, actual_cost, created_at").
				All()
			if err != nil {
				g.Log().Warningf(ctx, "export usage for tenant %d: %v", p.TenantID, err)
			} else {
				for _, rec := range result {
					logs = append(logs, rec.Map())
				}
			}
			exportData["usage"] = logs
		case "billing":
			var records []map[string]any
			result, err := dao.BilRecords.Ctx(ctx).Where("tenant_id", p.TenantID).
				OrderDesc("created_at").Limit(10000).
				Fields("id, relay_mode, model_name, input_tokens, output_tokens, total_cost, currency, status, created_at").
				All()
			if err != nil {
				g.Log().Warningf(ctx, "export billing for tenant %d: %v", p.TenantID, err)
			} else {
				for _, rec := range result {
					records = append(records, rec.Map())
				}
			}
			exportData["billing_records"] = records
		case "logs":
			var logs []map[string]any
			result, err := common.AuditModelCtx(ctx, "aud_operation_logs").Where("tenant_id", p.TenantID).
				OrderDesc("created_at").Limit(10000).
				Fields("id, action, resource_type, resource_id, ip_address, detail, created_at").
				All()
			if err != nil {
				g.Log().Warningf(ctx, "export logs for tenant %d: %v", p.TenantID, err)
			} else {
				for _, rec := range result {
					logs = append(logs, rec.Map())
				}
			}
			exportData["operation_logs"] = logs
		}
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal export data: %w", err)
	}

	// 通过 FileService 上传（未配对象存储时自动降级本地磁盘，保证导出功能基本可用）。
	// storage_path 必须保留 "exports/" 前缀——文件列表的 export 分类与保留期清理
	// （export_file_cleanup cron）均据此识别导出文件，故用 UploadWithKey 而非 Upload
	// （后者生成 "{date}/{uuid}" 路径会破坏该判据）。
	svc, err := common.NewFileServiceFromConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("初始化存储失败: %w", err)
	}
	storagePath := fmt.Sprintf("exports/tenant_%d/%s.json", p.TenantID, time.Now().Format("20060102_150405"))
	rec, err := svc.UploadWithKey(ctx, &common.FileUpload{
		Reader:      bytes.NewReader(jsonData),
		Filename:    fmt.Sprintf("export_tenant_%d_%s.json", p.TenantID, time.Now().Format("20060102")),
		ContentType: "application/json",
		Size:        int64(len(jsonData)),
		TenantID:    p.TenantID,
		UserID:      p.RequestedBy,
	}, storagePath)
	if err != nil {
		return nil, fmt.Errorf("upload export: %w", err)
	}

	// 下载链接：local 返回应用层 serve 相对 URL，OSS 返回预签名 URL（前端按是否 http 分支）。
	downloadURL, err := svc.GetDownloadURL(ctx, rec.ID)
	if err != nil {
		g.Log().Warningf(ctx, "data export: get download url for tenant %d: %v", p.TenantID, err)
	}

	g.Log().Infof(ctx, "data export completed for tenant %d, scopes=%v, size=%d bytes, provider=%s",
		p.TenantID, p.Scopes, len(jsonData), svc.ProviderName())
	return map[string]any{
		"file_id":          rec.ID,
		"storage_path":     storagePath,
		"storage_provider": svc.ProviderName(),
		"download_url":     downloadURL,
		"scopes":           p.Scopes,
		"size":             len(jsonData),
	}, nil
}

func handleDeletionRequest(ctx context.Context, payload json.RawMessage) (any, error) {
	var p DeletionPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}

	LogInfo(ctx, 0, fmt.Sprintf("开始处理租户 %d 的数据删除请求", p.TenantID))

	_, err := dao.TntUsers.Ctx(ctx).
		Where("tenant_id", p.TenantID).
		Data(do.TntUsers{
			DisplayName: "[deleted]",
			Email:       fmt.Sprintf("deleted_%d@deleted.local", p.TenantID),
		}).Update()
	if err != nil {
		LogError(ctx, 0, fmt.Sprintf("匿名化用户数据失败: %v", err))
		return nil, fmt.Errorf("anonymize users: %w", err)
	}

	if _, err := dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", p.TenantID).
		Data(do.ApiKeys{Status: "disabled"}).Update(); err != nil {
		return nil, fmt.Errorf("disable api keys: %w", err)
	}

	if _, err := common.AuditModelCtx(ctx, "aud_sensitive_access_logs").Where("tenant_id", p.TenantID).Delete(); err != nil {
		return nil, fmt.Errorf("delete sensitive access logs: %w", err)
	}

	if _, err := dao.TntTenants.Ctx(ctx).
		Where("id", p.TenantID).
		Data(do.TntTenants{Status: "terminated"}).Update(); err != nil {
		return nil, fmt.Errorf("terminate tenant: %w", err)
	}

	proof := map[string]any{
		"tenant_id":    p.TenantID,
		"reason":       p.Reason,
		"requested_by": p.RequestedBy,
		"completed_at": time.Now().Format(time.RFC3339),
		"actions":      []string{"用户数据已匿名化", "API Key 已禁用", "敏感日志已删除", "租户已标记 terminated"},
	}
	LogInfo(ctx, 0, fmt.Sprintf("数据删除完成: 租户%d", p.TenantID))
	return proof, nil
}

func handleExportCleanup(ctx context.Context, payload json.RawMessage) (any, error) {
	var p struct {
		FileID   int64 `json:"file_id"`
		TenantID int64 `json:"tenant_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	if _, err := dao.FilFiles.Ctx(ctx).Where("id", p.FileID).Delete(); err != nil {
		return nil, fmt.Errorf("delete export file %d: %w", p.FileID, err)
	}
	g.Log().Infof(ctx, "cleaned up export file %d for tenant %d", p.FileID, p.TenantID)
	return map[string]any{"file_id": p.FileID, "deleted": true}, nil
}
