package admin

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
)

// ErrorLogList returns a paginated list of system error logs.
func (s *sAdmin) ErrorLogList(ctx context.Context, req *v1.ErrorLogListReq) (*v1.ErrorLogListRes, error) {
	var conditions []string
	var args []any

	if req.Source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, req.Source)
	}
	if req.ErrorCode > 0 {
		conditions = append(conditions, "error_code = ?")
		args = append(args, req.ErrorCode)
	}
	if req.Resolved == "true" {
		conditions = append(conditions, "resolved = true")
	} else if req.Resolved == "false" {
		conditions = append(conditions, "resolved = false")
	}
	if req.StartDate != "" {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, req.StartDate)
	}
	if req.EndDate != "" {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, req.EndDate+" 23:59:59")
	}
	if req.Keyword != "" {
		conditions = append(conditions, "(error_message ILIKE ? OR request_path ILIKE ?)")
		kw := "%" + req.Keyword + "%"
		args = append(args, kw, kw)
	}

	where := strings.Join(conditions, " AND ")
	items, total, err := queryPage(ctx,
		"sys_error_logs", "*", where, "id DESC",
		req.Page, req.PageSize, args...)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []map[string]any{}
	}

	return &v1.ErrorLogListRes{
		List:     items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// ErrorLogDetail returns the detail of a single error log.
func (s *sAdmin) ErrorLogDetail(ctx context.Context, req *v1.ErrorLogDetailReq) (*v1.ErrorLogDetailRes, error) {
	result, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT * FROM sys_error_logs WHERE id = ?", req.Id)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, gerror.New("错误日志不存在")
	}
	row := result[0]
	data := make(map[string]any, len(row))
	for k, v := range row {
		data[k] = v.Val()
	}
	return &v1.ErrorLogDetailRes{Data: data}, nil
}

// ErrorLogResolve marks an error log as resolved.
func (s *sAdmin) ErrorLogResolve(ctx context.Context, req *v1.ErrorLogResolveReq) (*v1.ErrorLogResolveRes, error) {
	userID := getCtxUserID(ctx)
	_, err := dao.SysErrorLogs.Ctx(ctx).
		Where("id", req.Id).
		Data(g.Map{
			"resolved":    true,
			"resolved_by": userID,
			"resolved_at": gtime.Now(),
		}).Update()
	if err != nil {
		return nil, err
	}
	return &v1.ErrorLogResolveRes{}, nil
}

// ErrorLogBatchResolve marks multiple error logs as resolved.
func (s *sAdmin) ErrorLogBatchResolve(ctx context.Context, req *v1.ErrorLogBatchResolveReq) (*v1.ErrorLogBatchResolveRes, error) {
	userID := getCtxUserID(ctx)
	_, err := dao.SysErrorLogs.Ctx(ctx).
		WhereIn("id", req.Ids).
		Data(g.Map{
			"resolved":    true,
			"resolved_by": userID,
			"resolved_at": gtime.Now(),
		}).Update()
	if err != nil {
		return nil, err
	}
	return &v1.ErrorLogBatchResolveRes{}, nil
}

// ErrorLogStats returns error log statistics.
func (s *sAdmin) ErrorLogStats(ctx context.Context, _ *v1.ErrorLogStatsReq) (*v1.ErrorLogStatsRes, error) {
	// Total unresolved
	unresolved, err := dao.SysErrorLogs.Ctx(ctx).Where("resolved", false).Count()
	if err != nil {
		return nil, err
	}

	// Total today
	today := gtime.Now().Format("Y-m-d")
	todayCount, err := dao.SysErrorLogs.Ctx(ctx).
		Where("created_at >= ?", today).
		Count()
	if err != nil {
		return nil, err
	}

	// By source (last 7 days)
	bySourceResult, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT source, COUNT(*) as count FROM sys_error_logs WHERE created_at >= NOW() - INTERVAL '7 days' GROUP BY source")
	if err != nil {
		return nil, err
	}
	bySource := make([]map[string]any, 0, len(bySourceResult))
	for _, row := range bySourceResult {
		m := make(map[string]any, len(row))
		for k, v := range row {
			m[k] = v.Val()
		}
		bySource = append(bySource, m)
	}

	// By error code (last 7 days, top 10)
	byCodeResult, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT error_code, COUNT(*) as count FROM sys_error_logs WHERE created_at >= NOW() - INTERVAL '7 days' GROUP BY error_code ORDER BY count DESC LIMIT 10")
	if err != nil {
		return nil, err
	}
	byCode := make([]map[string]any, 0, len(byCodeResult))
	for _, row := range byCodeResult {
		m := make(map[string]any, len(row))
		for k, v := range row {
			m[k] = v.Val()
		}
		byCode = append(byCode, m)
	}

	return &v1.ErrorLogStatsRes{
		Data: map[string]any{
			"unresolved":    unresolved,
			"today_count":   todayCount,
			"by_source":     bySource,
			"by_error_code": byCode,
		},
	}, nil
}
