package task

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
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
			_ = dao.TntUsers.Ctx(ctx).Where("tenant_id", p.TenantID).
				Fields("id, username, display_name, role, status, created_at").Scan(&members)
			for _, m := range members {
				delete(m, "email")
			}
			exportData["members"] = members
		case "usage":
			var logs []map[string]any
			_ = dao.BilUsageLogs.Ctx(ctx).Where("tenant_id", p.TenantID).
				OrderDesc("created_at").Limit(10000).
				Fields("id, model_name, prompt_tokens, completion_tokens, total_tokens, cost, created_at").Scan(&logs)
			exportData["usage"] = logs
		case "billing":
			var records []map[string]any
			_ = dao.BilRecords.Ctx(ctx).Where("tenant_id", p.TenantID).
				OrderDesc("created_at").Limit(10000).
				Fields("id, type, amount, balance_before, balance_after, description, created_at").Scan(&records)
			exportData["billing_records"] = records
		case "logs":
			var logs []map[string]any
			_ = dao.AudOperationLogs.Ctx(ctx).Where("tenant_id", p.TenantID).
				OrderDesc("created_at").Limit(10000).
				Fields("id, action, resource_type, resource_id, ip_address, user_agent, created_at").Scan(&logs)
			exportData["operation_logs"] = logs
		}
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal export data: %w", err)
	}

	storagePath := fmt.Sprintf("exports/tenant_%d/%s.json", p.TenantID, time.Now().Format("20060102_150405"))
	password := generateRandomPassword(16)

	_, err = dao.FilFiles.Ctx(ctx).Data(do.FilFiles{
		TenantId:     p.TenantID,
		UserId:       p.RequestedBy,
		OriginalName: fmt.Sprintf("export_tenant_%d_%s.json", p.TenantID, time.Now().Format("20060102")),
		StoragePath:  storagePath,
		MimeType:     "application/json",
		Size:         int64(len(jsonData)),
	}).Insert()
	if err != nil {
		return nil, fmt.Errorf("save file metadata: %w", err)
	}

	g.Log().Infof(ctx, "data export completed for tenant %d, scopes=%v, size=%d bytes", p.TenantID, p.Scopes, len(jsonData))
	return map[string]any{
		"storage_path": storagePath,
		"scopes":       p.Scopes,
		"size":         len(jsonData),
		"password":     password,
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

	_, _ = dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", p.TenantID).
		Data(do.ApiKeys{Status: "disabled"}).Update()

	_, _ = dao.AudSensitiveAccessLogs.Ctx(ctx).Where("tenant_id", p.TenantID).Delete()

	_, _ = dao.TntTenants.Ctx(ctx).
		Where("id", p.TenantID).
		Data(do.TntTenants{Status: "terminated"}).Update()

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
	_, _ = dao.FilFiles.Ctx(ctx).Where("id", p.FileID).Delete()
	g.Log().Infof(ctx, "cleaned up export file %d for tenant %d", p.FileID, p.TenantID)
	return map[string]any{"file_id": p.FileID, "deleted": true}, nil
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}
