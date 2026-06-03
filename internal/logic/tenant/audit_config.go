package tenant

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"

	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
)

// getTenantSettings 读取租户的 settings JSONB 为 map。
func getTenantSettings(ctx context.Context, tenantID int64) (map[string]any, error) {
	var tenant *entity.TntTenants
	err := dao.TntTenants.Ctx(ctx).Where("id", tenantID).Fields("settings").Scan(&tenant)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if tenant == nil {
		return make(map[string]any), nil
	}
	settings := make(map[string]any)
	if tenant.Settings != "" {
		_ = json.Unmarshal([]byte(tenant.Settings), &settings)
	}
	return settings, nil
}

// saveTenantSettings 将 map 写回租户的 settings JSONB。
func saveTenantSettings(ctx context.Context, tenantID int64, settings map[string]any) error {
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = dao.TntTenants.Ctx(ctx).Where("id", tenantID).Data(do.TntTenants{
		Settings: string(settingsJSON),
	}).Update()
	return err
}

// AuditConfigGet returns the tenant's own audit level.
// 租户审计级别与全局级别完全独立，未设置时默认 masked。
func (s *sTenant) AuditConfigGet(ctx context.Context, req *v1.TenantAuditConfigGetReq) (*v1.TenantAuditConfigGetRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	tenantLevel := common.GetTenantAuditLevel(ctx, tenantID)
	if tenantLevel == "" {
		tenantLevel = common.AuditLevelMasked
	}

	return &v1.TenantAuditConfigGetRes{
		AuditLevel: tenantLevel,
	}, nil
}

// AuditConfigUpdate sets the audit level for a specific tenant.
// 租户可独立设置自己的审计级别，不受全局级别约束（双级别存储，各管各的）。
func (s *sTenant) AuditConfigUpdate(ctx context.Context, req *v1.TenantAuditConfigUpdateReq) (*v1.TenantAuditConfigUpdateRes, error) {
	if err := ownerOnly(ctx); err != nil {
		return nil, err
	}
	tenantID := middleware.GetTenantID(ctx)

	level := req.AuditLevel

	if !isValidAuditLevel(level) {
		return nil, common.NewBadRequestError("无效的审计级别，可选值：full, full_text, masked, question_only, none")
	}

	settings, err := getTenantSettings(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	settings["audit_level"] = level

	if err := saveTenantSettings(ctx, tenantID, settings); err != nil {
		return nil, err
	}

	return &v1.TenantAuditConfigUpdateRes{}, nil
}

// AuditLogs returns a paginated list of audit logs for the tenant.
func (s *sTenant) AuditLogs(ctx context.Context, req *v1.TenantAuditLogsReq) (*v1.TenantAuditLogsRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	// 使用原生 SQL 查询，绕过 GoFrame ScanAndCount 对 map[string]any 的 bug
	dataSQL := `SELECT id, tenant_id, user_id, user_type, action, resource_type, resource_id, detail, changes_json, ip_address, created_at
			 FROM aud_operation_logs WHERE tenant_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`
	result, err := common.GetAuditDB().Query(ctx, dataSQL, tenantID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}

	countResult, err := common.GetAuditDB().Query(ctx, "SELECT COUNT(*) AS total FROM aud_operation_logs WHERE tenant_id = ?", tenantID)
	if err != nil {
		return nil, err
	}
	total := 0
	if len(countResult) > 0 {
		total = countResult[0]["total"].Int()
	}

	items := make([]map[string]any, 0, len(result))
	for _, row := range result {
		m := make(map[string]any, len(row))
		for k, v := range row {
			m[k] = v.Val()
		}
		items = append(items, m)
	}

	return &v1.TenantAuditLogsRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// isValidAuditLevel checks if the given level string is a valid audit level.
func isValidAuditLevel(level string) bool {
	switch level {
	case common.AuditLevelFull, common.AuditLevelFullText, common.AuditLevelMasked,
		common.AuditLevelQuestionOnly, common.AuditLevelNone:
		return true
	default:
		return false
	}
}

// TenantRequestAuditLogs 分页查询租户的请求审计日志（不含 body，性能优先）
func (s *sTenant) TenantRequestAuditLogs(ctx context.Context, req *v1.TenantRequestAuditLogsReq) (*v1.TenantRequestAuditLogsRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var conditions []string
	var args []any

	conditions = append(conditions, "tenant_id = ?")
	args = append(args, tenantID)

	// 用户名筛选：先从主库查匹配的 user_id 集合，转为 IN 条件
	if req.Username != "" {
		type userRow struct {
			Id int64 `json:"id"`
		}
		var users []userRow
		err := g.DB().Ctx(ctx).Model("tnt_users").
			Where("tenant_id", tenantID).
			Where("username LIKE ?", "%"+req.Username+"%").
			Fields("id").
			Scan(&users)
		if err != nil {
			return nil, err
		}
		if len(users) == 0 {
			return &v1.TenantRequestAuditLogsRes{
				List: []map[string]any{}, Total: 0, Page: page, PageSize: pageSize,
			}, nil
		}
		userIDs := make([]int64, 0, len(users))
		for _, u := range users {
			userIDs = append(userIDs, u.Id)
		}
		conditions = append(conditions, "user_id IN (?)")
		args = append(args, userIDs)
	}
	if req.RequestId != "" {
		conditions = append(conditions, "request_id = ?")
		args = append(args, req.RequestId)
	}
	if req.TaskId != "" {
		conditions = append(conditions, "task_id = ?")
		args = append(args, req.TaskId)
	}
	if req.Path != "" {
		conditions = append(conditions, "path LIKE ?")
		args = append(args, "%"+req.Path+"%")
	}
	if req.StatusCode > 0 {
		conditions = append(conditions, "status_code = ?")
		args = append(args, req.StatusCode)
	}
	if req.StartDate != "" {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, req.StartDate+" 00:00:00")
	}
	if req.EndDate != "" {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, req.EndDate+" 23:59:59")
	}

	where := strings.Join(conditions, " AND ")

	// Step 1: 从审计库查询审计记录（不带 JOIN）
	auditDB := common.GetAuditDB()
	countM := auditDB.Ctx(ctx).Model("aud_request_logs").Safe()
	if where != "" {
		countM = countM.Where(where, args...)
	}
	total, err := countM.Count()
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return &v1.TenantRequestAuditLogsRes{
			List: []map[string]any{}, Total: 0, Page: page, PageSize: pageSize,
		}, nil
	}

	dataM := auditDB.Ctx(ctx).Model("aud_request_logs").Safe().
		Fields("id, request_id, user_id, project_id, method, path, query_params, status_code, client_ip, user_agent, latency_ms, first_token_ms, tenant_audit_level as audit_level, task_id, task_status, task_completed_at, created_at").
		OrderDesc("created_at").
		Page(page, pageSize)
	if where != "" {
		dataM = dataM.Where(where, args...)
	}
	result, err := dataM.All()
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(result))
	for _, row := range result {
		m := make(map[string]any, len(row))
		for k, v := range row {
			m[k] = v.Val()
		}
		items = append(items, m)
	}

	// Step 2: 从主库批量查询关联信息并合并
	common.EnrichAuditRecords(ctx, items)

	return &v1.TenantRequestAuditLogsRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
} // TenantRequestAuditLogDetail 查询单条请求审计日志详情（含 request_body 和 response_body）
func (s *sTenant) TenantRequestAuditLogDetail(ctx context.Context, req *v1.TenantRequestAuditLogDetailReq) (*v1.TenantRequestAuditLogDetailRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	var record *entity.AudRequestLogs
	err := common.AuditModelCtx(ctx, "aud_request_logs").
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&record)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if record == nil {
		return nil, common.NewNotFoundError("请求审计日志")
	}

	// 租户只看到自己级别的数据：用 tenant_* 字段替换主字段
	b, _ := json.Marshal(record)
	var detail map[string]any
	_ = json.Unmarshal(b, &detail)
	detail["request_body"] = record.TenantRequestBody
	detail["response_body"] = record.TenantResponseBody
	detail["audit_level"] = record.TenantAuditLevel
	delete(detail, "tenant_request_body")
	delete(detail, "tenant_response_body")
	delete(detail, "tenant_audit_level")
	// 请求头/响应头仅管理后台可见，租户不可见
	delete(detail, "request_headers")
	delete(detail, "response_headers")
	// 异步任务上游响应头仅管理后台可见
	delete(detail, "task_upstream_headers")
	return &v1.TenantRequestAuditLogDetailRes{Data: detail}, nil
}
