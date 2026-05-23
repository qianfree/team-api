package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/admin/v1"
)

// ChannelErrorEventList 渠道错误事件列表
func (s *sAdmin) ChannelErrorEventList(ctx context.Context, req *v1.ChannelErrorEventListReq) (*v1.ChannelErrorEventListRes, error) {
	var conditions []string
	var args []any

	if req.ChannelID > 0 {
		conditions = append(conditions, "channel_id = ?")
		args = append(args, req.ChannelID)
	}
	if req.ErrorCategory != "" {
		conditions = append(conditions, "error_category = ?")
		args = append(args, req.ErrorCategory)
	}
	if req.StatusCode > 0 {
		conditions = append(conditions, "status_code = ?")
		args = append(args, req.StatusCode)
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
		conditions = append(conditions, "(error_message ILIKE ? OR channel_name ILIKE ? OR model_name ILIKE ?)")
		kw := "%" + req.Keyword + "%"
		args = append(args, kw, kw, kw)
	}

	where := strings.Join(conditions, " AND ")
	items, total, err := queryPage(ctx,
		"chn_error_events",
		"id, channel_id, channel_name, provider, model_name, error_category, status_code, error_message, is_retryable, attempt, is_final, latency_ms, request_id, created_at",
		where, "created_at DESC",
		req.Page, req.PageSize, args...)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []map[string]any{}
	}

	return &v1.ChannelErrorEventListRes{
		List:     items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// ChannelErrorStats 渠道错误统计
func (s *sAdmin) ChannelErrorStats(ctx context.Context, req *v1.ChannelErrorStatsReq) (*v1.ChannelErrorStatsRes, error) {
	timeCondition := fmt.Sprintf("created_at >= NOW() - INTERVAL '%d hours'", req.Hours)
	var args []any
	if req.ChannelID > 0 {
		timeCondition += " AND channel_id = ?"
		args = append(args, req.ChannelID)
	}

	// 总数
	totalRecord, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT COUNT(*) as total FROM chn_error_events WHERE "+timeCondition, args...)
	if err != nil {
		return nil, err
	}
	total := 0
	if len(totalRecord) > 0 {
		total = totalRecord[0]["total"].Int()
	}

	// 按分类
	byCategory, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT error_category, COUNT(*) as count FROM chn_error_events WHERE "+timeCondition+
			" GROUP BY error_category ORDER BY count DESC", args...)
	if err != nil {
		return nil, err
	}

	// 按状态码
	byStatusCode, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT status_code, COUNT(*) as count FROM chn_error_events WHERE "+timeCondition+
			" GROUP BY status_code ORDER BY count DESC", args...)
	if err != nil {
		return nil, err
	}

	// Top 渠道
	topChannels, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT channel_id, channel_name, provider, COUNT(*) as error_count FROM chn_error_events WHERE "+timeCondition+
			" GROUP BY channel_id, channel_name, provider ORDER BY error_count DESC LIMIT 5", args...)
	if err != nil {
		return nil, err
	}

	return &v1.ChannelErrorStatsRes{
		Total:        total,
		ByCategory:   toMapList(byCategory),
		ByStatusCode: toMapList(byStatusCode),
		TopChannels:  toMapList(topChannels),
	}, nil
}

// ChannelErrorTrend 渠道错误趋势
func (s *sAdmin) ChannelErrorTrend(ctx context.Context, req *v1.ChannelErrorTrendReq) (*v1.ChannelErrorTrendRes, error) {
	interval := "1 hour"
	if req.Hours <= 6 {
		interval = "30 minutes"
	} else if req.Hours <= 24 {
		interval = "1 hour"
	} else {
		interval = "6 hours"
	}

	timeCondition := fmt.Sprintf("created_at >= NOW() - INTERVAL '%d hours'", req.Hours)
	var args []any
	if req.ChannelID > 0 {
		timeCondition += " AND channel_id = ?"
		args = append(args, req.ChannelID)
	}
	if req.Category != "" {
		timeCondition += " AND error_category = ?"
		args = append(args, req.Category)
	}

	bucketExpr := fmt.Sprintf("date_trunc('%s', created_at) AS bucket", interval)
	result, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT "+bucketExpr+", error_category, COUNT(*) as count FROM chn_error_events WHERE "+timeCondition+
			" GROUP BY bucket, error_category ORDER BY bucket", args...)
	if err != nil {
		return nil, err
	}

	return &v1.ChannelErrorTrendRes{
		Points: toMapList(result),
	}, nil
}

// ChannelErrorTopChannels 错误最多的渠道
func (s *sAdmin) ChannelErrorTopChannels(ctx context.Context, req *v1.ChannelErrorTopChannelsReq) (*v1.ChannelErrorTopChannelsRes, error) {
	timeFilter := fmt.Sprintf("created_at >= NOW() - INTERVAL '%d hours'", req.Hours)
	result, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT channel_id, channel_name, provider, COUNT(*) as error_count, "+
			"COUNT(*) FILTER (WHERE error_category = 'rate_limit') as rate_limit_count, "+
			"COUNT(*) FILTER (WHERE error_category = 'server_error') as server_error_count, "+
			"COUNT(*) FILTER (WHERE error_category = 'timeout') as timeout_count "+
			"FROM chn_error_events WHERE "+timeFilter+
			" GROUP BY channel_id, channel_name, provider ORDER BY error_count DESC LIMIT ?",
		req.Limit)
	if err != nil {
		return nil, err
	}

	return &v1.ChannelErrorTopChannelsRes{
		List: toMapList(result),
	}, nil
}

// ChannelErrorCategories 错误分类选项
func (s *sAdmin) ChannelErrorCategories(ctx context.Context, req *v1.ChannelErrorCategoriesReq) (*v1.ChannelErrorCategoriesRes, error) {
	return &v1.ChannelErrorCategoriesRes{
		Data: []map[string]string{
			{"value": "rate_limit", "label": "频率限制"},
			{"value": "auth_error", "label": "认证错误"},
			{"value": "timeout", "label": "超时"},
			{"value": "upstream_error", "label": "上游错误"},
			{"value": "server_error", "label": "服务端错误"},
			{"value": "network_error", "label": "网络错误"},
			{"value": "unknown", "label": "未知错误"},
		},
	}, nil
}

// toMapList 将 Result 转为 []map[string]any
func toMapList(result gdb.Result) []map[string]any {
	if result == nil || len(result) == 0 {
		return []map[string]any{}
	}
	list := make([]map[string]any, len(result))
	for i, row := range result {
		data := make(map[string]any, len(row))
		for k, v := range row {
			data[k] = v.Val()
		}
		list[i] = data
	}
	return list
}
