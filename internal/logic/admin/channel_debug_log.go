package admin

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
)

// debugLogListFields 列表查询字段：元数据 + 四段体积（octet_length），绝不 SELECT body 列
const debugLogListFields = "id, channel_id, channel_name, channel_type, request_id, tenant_id, user_id, api_key_id, " +
	"model_name, upstream_model, relay_mode, inbound_path, upstream_url, is_stream, retry_index, is_final, " +
	"upstream_status_code, client_status_code, error, upstream_latency_ms, total_latency_ms, first_token_ms, conversion, " +
	"octet_length(client_req_body) AS client_req_body_size, " +
	"octet_length(upstream_req_body) AS upstream_req_body_size, " +
	"octet_length(upstream_resp_body) AS upstream_resp_body_size, " +
	"octet_length(client_resp_body) AS client_resp_body_size, " +
	"created_at"

// ChannelDebugLogList 渠道调试日志列表（原生 SQL：octet_length 表达式经 gdb Fields 有引号化风险）
func (s *sAdmin) ChannelDebugLogList(ctx context.Context, req *v1.ChannelDebugLogListReq) (*v1.ChannelDebugLogListRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	conditions := []string{"channel_id = ?"}
	args := []any{req.ChannelID}
	if req.RequestID != "" {
		conditions = append(conditions, "request_id = ?")
		args = append(args, req.RequestID)
	}
	if req.ModelName != "" {
		conditions = append(conditions, "model_name ILIKE ?")
		args = append(args, "%"+req.ModelName+"%")
	}
	if req.UpstreamStatus != nil {
		conditions = append(conditions, "upstream_status_code = ?")
		args = append(args, *req.UpstreamStatus)
	}
	if req.ClientStatus != nil {
		conditions = append(conditions, "client_status_code = ?")
		args = append(args, *req.ClientStatus)
	}
	if req.IsStream != nil {
		conditions = append(conditions, "is_stream = ?")
		args = append(args, *req.IsStream)
	}
	if req.OnlyError != nil && *req.OnlyError {
		conditions = append(conditions, "COALESCE(error, '') <> ''")
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

	countRows, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT COUNT(*) AS total FROM chn_debug_logs WHERE "+where, args...)
	if err != nil {
		return nil, err
	}
	total := 0
	if len(countRows) > 0 {
		total = countRows[0]["total"].Int()
	}
	if total == 0 {
		return &v1.ChannelDebugLogListRes{List: []map[string]any{}, Total: 0, Page: req.Page, PageSize: req.PageSize}, nil
	}

	rows, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT "+debugLogListFields+" FROM chn_debug_logs WHERE "+where+
			" ORDER BY id DESC LIMIT ? OFFSET ?",
		append(args, req.PageSize, (req.Page-1)*req.PageSize)...)
	if err != nil {
		return nil, err
	}

	return &v1.ChannelDebugLogListRes{
		List:     toMapList(rows),
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// ChannelDebugLogStats 渠道调试日志统计（条数/落库体积/最早时间，供清理提示）
func (s *sAdmin) ChannelDebugLogStats(ctx context.Context, req *v1.ChannelDebugLogStatsReq) (*v1.ChannelDebugLogStatsRes, error) {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT COUNT(*) AS total, "+
			"COALESCE(SUM(octet_length(client_req_body) + octet_length(upstream_req_body) + "+
			"octet_length(upstream_resp_body) + octet_length(client_resp_body)), 0) AS total_bytes, "+
			"MIN(created_at) AS oldest_at "+
			"FROM chn_debug_logs WHERE channel_id = ?", req.ChannelID)
	if err != nil {
		return nil, err
	}
	res := &v1.ChannelDebugLogStatsRes{}
	if len(rows) > 0 {
		res.Total = rows[0]["total"].Int64()
		res.TotalBytes = rows[0]["total_bytes"].Int64()
		if v := rows[0]["oldest_at"]; v != nil {
			if t, ok := v.Interface().(time.Time); ok && !t.IsZero() {
				res.OldestAt = gtime.New(t).String()
			}
		}
	}
	return res, nil
}

// ChannelDebugLogDetail 渠道调试日志详情（四段完整报文；headers JSONB 解析为对象便于前端渲染）
func (s *sAdmin) ChannelDebugLogDetail(ctx context.Context, req *v1.ChannelDebugLogDetailReq) (*v1.ChannelDebugLogDetailRes, error) {
	record, err := dao.ChnDebugLogs.Ctx(ctx).
		Where("channel_id", req.ChannelID).
		Where("id", req.ID).
		One()
	if err != nil {
		return nil, err
	}
	if record.IsEmpty() {
		return nil, common.NewNotFoundError("调试日志")
	}

	data := record.Map()
	// headers JSONB 列可能以字符串返回，解析为对象；无效/空值保持原样
	for _, field := range []string{
		"client_req_headers", "upstream_req_headers", "upstream_resp_headers", "client_resp_headers",
	} {
		data[field] = parseJSONField(data[field])
	}
	return &v1.ChannelDebugLogDetailRes{Data: data}, nil
}

// ChannelDebugLogDelete 删除单条调试日志（硬删除）
func (s *sAdmin) ChannelDebugLogDelete(ctx context.Context, req *v1.ChannelDebugLogDeleteReq) (*v1.ChannelDebugLogDeleteRes, error) {
	_, err := dao.ChnDebugLogs.Ctx(ctx).
		Where("channel_id", req.ChannelID).
		Where("id", req.ID).
		Delete()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// ChannelDebugLogClear 清空渠道调试日志（按渠道硬删除全部）
func (s *sAdmin) ChannelDebugLogClear(ctx context.Context, req *v1.ChannelDebugLogClearReq) (*v1.ChannelDebugLogClearRes, error) {
	_, err := dao.ChnDebugLogs.Ctx(ctx).
		Where("channel_id", req.ChannelID).
		Delete()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// parseJSONField 尝试将字符串形式的 JSON 解析为对象；解析失败或非字符串时原样返回
func parseJSONField(v any) any {
	s, ok := v.(string)
	if !ok || s == "" || s == "null" {
		return v
	}
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return v
	}
	return parsed
}
