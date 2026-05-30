package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/utility/export"
)

// audit levels — 委托给 common 包的常量
const (
	AuditLevelFull         = common.AuditLevelFull
	AuditLevelFullText     = common.AuditLevelFullText
	AuditLevelMasked       = common.AuditLevelMasked
	AuditLevelQuestionOnly = common.AuditLevelQuestionOnly
	AuditLevelNone         = common.AuditLevelNone
)

// validAuditLevels defines the set of allowed audit level values.
var validAuditLevels = map[string]bool{
	AuditLevelFull:         true,
	AuditLevelFullText:     true,
	AuditLevelMasked:       true,
	AuditLevelQuestionOnly: true,
	AuditLevelNone:         true,
}

// GetAuditConfig retrieves the global audit level from sys_options.
// Package-level wrapper for use by tenant package.
func GetAuditConfig(ctx context.Context) (string, error) {
	return common.GetAuditLevel(ctx), nil
}

// GetAuditConfig retrieves the global audit level from sys_options.
func (s *sAdmin) GetAuditConfig(ctx context.Context, _ *v1.AuditConfigGetReq) (*v1.AuditConfigGetRes, error) {
	return &v1.AuditConfigGetRes{AuditLevel: common.GetAuditLevel(ctx)}, nil
}

// UpdateAuditConfig updates the global audit level.
func (s *sAdmin) UpdateAuditConfig(ctx context.Context, req *v1.AuditConfigUpdateReq) (*v1.AuditConfigUpdateRes, error) {
	if !validAuditLevels[req.AuditLevel] {
		return nil, common.NewBadRequestError("无效的审计级别，可选值：full, full_text, masked, question_only, none")
	}

	// Upsert into sys_options
	count, err := dao.SysOptions.Ctx(ctx).
		Where("key", "audit_level").Count()
	if err != nil {
		return nil, err
	}

	if count > 0 {
		_, err = dao.SysOptions.Ctx(ctx).
			Where("key", "audit_level").
			Data(do.SysOptions{
				Value: req.AuditLevel,
			}).
			Update()
	} else {
		_, err = dao.SysOptions.Ctx(ctx).
			Data(do.SysOptions{
				Key:         "audit_level",
				Value:       req.AuditLevel,
				Description: "全局审计级别",
				Category:    "security",
				IsPublic:    false,
			}).
			Insert()
	}
	if err != nil {
		return nil, err
	}

	// Invalidate cache
	common.Config().SetOption(ctx, "audit_level", req.AuditLevel)
	return nil, nil
}

// queryPage 使用 GoFrame ORM 分页查询，返回 []map[string]any。
// 分开调用 Count 和 All 避免 ScanAndCount 对 map[string]any 的 bug。
func queryPage(ctx context.Context, table, fields, where, orderBy string, page, pageSize int, args ...any) ([]map[string]any, int, error) {
	m := g.DB().Ctx(ctx).Model(table).Safe()
	if where != "" {
		m = m.Where(where, args...)
	}

	total, err := m.Count()
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []map[string]any{}, 0, nil
	}

	// 重新构建查询（Count 会消耗 Model 状态）
	q := g.DB().Ctx(ctx).Model(table).Safe()
	if fields != "" {
		q = q.Fields(fields)
	}
	if where != "" {
		q = q.Where(where, args...)
	}
	if orderBy != "" {
		q = q.Order(orderBy)
	}

	result, err := q.Page(page, pageSize).All()
	if err != nil {
		return nil, 0, err
	}

	items := make([]map[string]any, 0, len(result))
	for _, row := range result {
		m := make(map[string]any, len(row))
		for k, v := range row {
			m[k] = v.Val()
		}
		items = append(items, m)
	}
	return items, total, nil
}

// ListOperationLogs retrieves a paginated list of operation logs with optional filters.
func (s *sAdmin) ListOperationLogs(ctx context.Context, req *v1.OperationLogListReq) (*v1.OperationLogListRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var conditions []string
	var args []any

	if req.UserID > 0 {
		conditions = append(conditions, "user_id = ?")
		args = append(args, int64(req.UserID))
	}
	if req.UserType != "" {
		conditions = append(conditions, "user_type = ?")
		args = append(args, req.UserType)
	}
	if req.Action != "" {
		conditions = append(conditions, "action LIKE ?")
		args = append(args, "%"+req.Action+"%")
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
	items, total, err := queryPage(ctx,
		"aud_operation_logs",
		"id, tenant_id, user_id, user_type, action, resource_type, resource_id, detail, changes_json, ip_address, created_at",
		where, "created_at DESC", page, pageSize, args...)
	if err != nil {
		return nil, err
	}

	return &v1.OperationLogListRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ListSensitiveAccessLogs retrieves a paginated list of sensitive data access logs.
func (s *sAdmin) ListSensitiveAccessLogs(ctx context.Context, req *v1.SensitiveLogListReq) (*v1.SensitiveLogListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var conditions []string
	var args []any

	if req.UserID > 0 {
		conditions = append(conditions, "user_id = ?")
		args = append(args, int64(req.UserID))
	}
	if req.ResourceType != "" {
		conditions = append(conditions, "resource_type = ?")
		args = append(args, req.ResourceType)
	}

	where := strings.Join(conditions, " AND ")
	items, total, err := queryPage(ctx,
		"aud_sensitive_access_logs",
		"id, user_id, user_type, resource_type, resource_id, action, reason, ip_address, user_agent, created_at",
		where, "created_at DESC", page, pageSize, args...)
	if err != nil {
		return nil, err
	}

	return &v1.SensitiveLogListRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// LogSensitiveAccess inserts a sensitive data access log entry.
func LogSensitiveAccess(ctx context.Context, userID int64, userType, resourceType string, resourceID int64, action, reason, ip, userAgent string) error {
	_, err := dao.AudSensitiveAccessLogs.Ctx(ctx).Insert(do.AudSensitiveAccessLogs{
		UserId:       userID,
		UserType:     userType,
		ResourceType: resourceType,
		ResourceId:   resourceID,
		Action:       action,
		Reason:       reason,
		IpAddress:    ip,
		UserAgent:    userAgent,
	})
	if err != nil {
		return gerror.Wrapf(err, "记录敏感数据访问日志失败")
	}
	return nil
}

// MaskSensitiveData 委托给 common.MaskSensitiveData
func MaskSensitiveData(data string) string {
	return common.MaskSensitiveData(data)
}

// ListRequestAuditLogs 分页查询请求审计日志（不返回 request_body/response_body 以优化性能）
func (s *sAdmin) ListRequestAuditLogs(ctx context.Context, req *v1.RequestAuditLogListReq) (*v1.RequestAuditLogListRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var conditions []string
	var args []any

	if req.TenantID > 0 {
		conditions = append(conditions, "a.tenant_id = ?")
		args = append(args, int64(req.TenantID))
	}
	if req.ApiKeyID > 0 {
		conditions = append(conditions, "a.api_key_id = ?")
		args = append(args, int64(req.ApiKeyID))
	}
	if req.Username != "" {
		conditions = append(conditions, "t.username LIKE ?")
		args = append(args, "%"+req.Username+"%")
	}
	if req.RequestId != "" {
		conditions = append(conditions, "a.request_id = ?")
		args = append(args, req.RequestId)
	}
	if req.TaskId != "" {
		conditions = append(conditions, "a.task_id = ?")
		args = append(args, req.TaskId)
	}
	if req.Method != "" {
		conditions = append(conditions, "a.method = ?")
		args = append(args, req.Method)
	}
	if req.Path != "" {
		conditions = append(conditions, "a.path LIKE ?")
		args = append(args, "%"+req.Path+"%")
	}
	if req.StatusCode > 0 {
		conditions = append(conditions, "a.status_code = ?")
		args = append(args, req.StatusCode)
	}
	if req.StartDate != "" {
		conditions = append(conditions, "a.created_at >= ?")
		args = append(args, req.StartDate+" 00:00:00")
	}
	if req.EndDate != "" {
		conditions = append(conditions, "a.created_at <= ?")
		args = append(args, req.EndDate+" 23:59:59")
	}

	where := strings.Join(conditions, " AND ")
	fromClause := "aud_request_logs a LEFT JOIN tnt_users t ON a.user_id = t.id AND a.tenant_id = t.tenant_id LEFT JOIN tnt_projects p ON a.project_id = p.id LEFT JOIN tnt_tenants tn ON a.tenant_id = tn.id LEFT JOIN api_keys ak ON a.api_key_id = ak.id"
	whereClause := ""
	if where != "" {
		whereClause = " WHERE " + where
	}

	countSQL := "SELECT COUNT(*) AS total FROM " + fromClause + whereClause
	countResult, err := g.DB().Ctx(ctx).Query(ctx, countSQL, args...)
	if err != nil {
		return nil, err
	}
	total := 0
	if len(countResult) > 0 {
		total = countResult[0]["total"].Int()
	}

	dataSQL := fmt.Sprintf(
		`SELECT a.id, a.tenant_id, COALESCE(tn.name, '') AS tenant_name, a.user_id, COALESCE(t.username, '') AS username, a.project_id, COALESCE(p.name, '') AS project_name, a.api_key_id, COALESCE(ak.name, '') AS api_key_name, a.request_id, a.method, a.path, a.query_params, a.status_code, a.client_ip, a.user_agent, a.latency_ms, a.first_token_ms, a.created_at, a.updated_at, a.audit_level
		 FROM %s%s ORDER BY a.created_at DESC LIMIT ? OFFSET ?`,
		fromClause, whereClause,
	)
	result, err := g.DB().Ctx(ctx).Query(ctx, dataSQL, append(args, pageSize, (page-1)*pageSize)...)
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

	return &v1.RequestAuditLogListRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetRequestAuditLogDetail 查询单条请求审计日志详情（含完整 request_body 和 response_body）
func (s *sAdmin) GetRequestAuditLogDetail(ctx context.Context, req *v1.RequestAuditLogDetailReq) (*v1.RequestAuditLogDetailRes, error) {
	var record *entity.AudRequestLogs
	err := dao.AudRequestLogs.Ctx(ctx).
		Where("id", req.Id).
		Scan(&record)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, common.NewNotFoundError("审计日志")
	}
	b, _ := json.Marshal(record)
	var detail map[string]any
	_ = json.Unmarshal(b, &detail)
	return &v1.RequestAuditLogDetailRes{Data: detail}, nil
}

// ExportOperationLogs exports operation logs to CSV or Excel.
func (s *sAdmin) ExportOperationLogs(ctx context.Context, req *v1.OperationLogExportReq) (*v1.OperationLogExportRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	buildOpLogWhere := func() (string, []any) {
		var conditions []string
		var args []any
		if req.UserID > 0 {
			conditions = append(conditions, "user_id = ?")
			args = append(args, int64(req.UserID))
		}
		if req.UserType != "" {
			conditions = append(conditions, "user_type = ?")
			args = append(args, req.UserType)
		}
		if req.Action != "" {
			conditions = append(conditions, "action LIKE ?")
			args = append(args, "%"+req.Action+"%")
		}
		if req.StartDate != "" {
			conditions = append(conditions, "created_at >= ?")
			args = append(args, req.StartDate+" 00:00:00")
		}
		if req.EndDate != "" {
			conditions = append(conditions, "created_at <= ?")
			args = append(args, req.EndDate+" 23:59:59")
		}
		where := ""
		if len(conditions) > 0 {
			where = strings.Join(conditions, " AND ")
		}
		return where, args
	}

	selectFields := "id, user_id, user_type, action, resource_type, resource_id, ip_address, detail, created_at"

	config := export.Config{
		Format:   req.Format,
		Filename: "操作日志_" + gtime.Now().Format("Ymd_His"),
		Columns: []export.Column{
			{Field: "id", Header: "ID"},
			{Field: "user_id", Header: "用户ID"},
			{Field: "user_type", Header: "用户类型"},
			{Field: "action", Header: "操作"},
			{Field: "resource_type", Header: "目标类型"},
			{Field: "resource_id", Header: "目标ID"},
			{Field: "ip_address", Header: "IP地址"},
			{Field: "detail", Header: "详情"},
			{Field: "created_at", Header: "创建时间"},
		},
	}

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			where, args := buildOpLogWhere()
			items, _, err := queryPage(ctx, "aud_operation_logs", selectFields, where, "created_at DESC", offset/1000+1, 1000, args...)
			if err != nil {
				return
			}
			for _, item := range items {
				createdAt := ""
				if v, ok := item["created_at"]; ok {
					createdAt = fmt.Sprintf("%v", v)
				}
				if !yield(map[string]any{
					"id":            item["id"],
					"user_id":       item["user_id"],
					"user_type":     item["user_type"],
					"action":        item["action"],
					"resource_type": item["resource_type"],
					"resource_id":   item["resource_id"],
					"ip_address":    item["ip_address"],
					"detail":        item["detail"],
					"created_at":    createdAt,
				}) {
					return
				}
			}
			if len(items) < 1000 {
				break
			}
			offset += 1000
		}
	})
}

// ContentFilterLogList returns a paginated list of content filter interception logs.
func (s *sAdmin) ContentFilterLogList(ctx context.Context, req *v1.ContentFilterLogListReq) (*v1.ContentFilterLogListRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var conditions []string
	var args []any

	if req.TenantID > 0 {
		conditions = append(conditions, "l.tenant_id = ?")
		args = append(args, int64(req.TenantID))
	}
	if req.Mode != "" {
		conditions = append(conditions, "l.filter_mode = ?")
		args = append(args, req.Mode)
	}
	if req.Blocked == "true" {
		conditions = append(conditions, "l.blocked = true")
	} else if req.Blocked == "false" {
		conditions = append(conditions, "l.blocked = false")
	}
	if req.StartDate != "" {
		conditions = append(conditions, "l.created_at >= ?")
		args = append(args, req.StartDate+" 00:00:00")
	}
	if req.EndDate != "" {
		conditions = append(conditions, "l.created_at <= ?")
		args = append(args, req.EndDate+" 23:59:59")
	}
	if req.Keyword != "" {
		conditions = append(conditions, "(l.matched_words::text ILIKE ? OR l.path ILIKE ?)")
		kw := "%" + req.Keyword + "%"
		args = append(args, kw, kw)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	fields := `l.id, l.tenant_id, l.user_id, l.api_key_id, l.project_id, l.request_id, l.method, l.path, l.client_ip,
		l.filter_mode, l.matched_words, l.original_snippet, l.blocked, l.created_at,
		t.name AS tenant_name, u.username AS user_name, k.name AS api_key_name, p.name AS project_name`

	// Count
	countSQL := "SELECT COUNT(*) AS total FROM aud_content_filter_logs l" + where
	countResult, err := g.DB().Ctx(ctx).Query(ctx, countSQL, args...)
	if err != nil {
		return nil, err
	}
	total := 0
	if len(countResult) > 0 {
		total = countResult[0]["total"].Int()
	}

	// Query with JOINs
	dataSQL := fmt.Sprintf(`SELECT %s FROM aud_content_filter_logs l
		LEFT JOIN tnt_tenants t ON l.tenant_id = t.id
		LEFT JOIN tnt_users u ON l.user_id = u.id
		LEFT JOIN api_keys k ON l.api_key_id = k.id
		LEFT JOIN tnt_projects p ON l.project_id = p.id
		%s ORDER BY l.created_at DESC LIMIT ? OFFSET ?`, fields, where)

	result, err := g.DB().Ctx(ctx).Query(ctx, dataSQL, append(args, pageSize, (page-1)*pageSize)...)
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

	return &v1.ContentFilterLogListRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
