package aireqanalyze

import (
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"
)

// API 请求/响应结构与参数解析（插件路由不走 gf gen ctrl 链路，手动解析）。

// maxAnalysisSpanDays 单次分析窗口上限（防误操作全量扫描大时间跨度）。
const maxAnalysisSpanDays = 31

// ── 发起分析 ──

// analyzeParams 发起按需分析的入参。
type analyzeParams struct {
	TenantID  int64
	UserID    int64
	TimeStart time.Time
	TimeEnd   time.Time
}

func parseAnalyzeParams(r *ghttp.Request) (*analyzeParams, error) {
	p := &analyzeParams{
		TenantID: r.Get("tenant_id").Int64(),
		UserID:   r.Get("user_id").Int64(), // 0 = 租户全员
	}
	if p.TenantID <= 0 {
		return nil, gerror.New("请指定要分析的租户")
	}

	var err error
	if p.TimeStart, err = parseTimeBound(r.Get("time_start").String()); err != nil {
		return nil, gerror.New("开始时间格式应为 YYYY-MM-DD HH:mm:ss 或 YYYY-MM-DD")
	}
	if p.TimeEnd, err = parseEndBound(r.Get("time_end").String()); err != nil {
		return nil, gerror.New("结束时间格式应为 YYYY-MM-DD HH:mm:ss 或 YYYY-MM-DD")
	}
	if !p.TimeStart.Before(p.TimeEnd) {
		return nil, gerror.New("开始时间必须早于结束时间")
	}
	if p.TimeEnd.Sub(p.TimeStart) > maxAnalysisSpanDays*24*time.Hour {
		return nil, gerror.Newf("分析窗口一次最多 %d 天，请分段发起", maxAnalysisSpanDays)
	}
	return p, nil
}

// parseTimeBound 解析时间边界，支持三种常用格式；日期粒度自动补到当天边界之外
// （开始=00:00:00 由字面量自带，结束若只有日期则补到 23:59:59 由调用方处理）。
func parseTimeBound(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []struct {
		layout string
		isDate bool
	}{
		{"2006-01-02 15:04:05", false},
		{"2006-01-02 15:04", false},
		{"2006-01-02", true},
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l.layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, gerror.New("invalid time")
}

// parseEndBound 解析结束时间：日期粒度补到当天 23:59:59。
func parseEndBound(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t.Add(24*time.Hour - time.Second), nil
	}
	return parseTimeBound(s)
}

// ── 路径参数 ──

// pathID 取 {id} 路径参数。
func pathID(r *ghttp.Request) (int64, error) {
	id, err := strconv.ParseInt(r.Get("id").String(), 10, 64)
	if err != nil || id <= 0 {
		return 0, gerror.New("无效的资源 ID")
	}
	return id, nil
}

// ── 响应结构 ──

// AnalyzeRes 发起分析响应。
type AnalyzeRes struct {
	JobId int64 `json:"job_id"`
}

// ReportListItem 报告列表项（含租户/用户名联查字段）。
type ReportListItem struct {
	ReportEntity
	TenantName     string `json:"tenant_name"`
	TargetUsername string `json:"target_username"`
}

// ReportDetailRes 报告详情。
type ReportDetailRes struct {
	ReportEntity
	TenantName     string          `json:"tenant_name"`
	TargetUsername string          `json:"target_username"`
	Findings       []FindingEntity `json:"findings"`
}

// ListRes 通用分页响应。
type ListRes struct {
	List     any `json:"list"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}
