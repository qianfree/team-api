package aianalysis

import (
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// 发起分析的时间窗参数解析（管理后台 API 层调用，见 internal/logic/admin/ai_analysis.go）。

// MaxAnalysisSpanDays 单次分析窗口上限（防误操作全量扫描大时间跨度）。
const MaxAnalysisSpanDays = 31

// AnalysisParams 发起按需分析的入参（时间窗已解析）。
type AnalysisParams struct {
	TenantID  int64
	UserID    int64 // 0 = 租户全员
	TimeStart time.Time
	TimeEnd   time.Time
}

// ParseAnalysisParams 校验并解析发起分析的参数：租户必填、时间窗格式/顺序/跨度校验。
func ParseAnalysisParams(tenantID, userID int64, timeStart, timeEnd string) (*AnalysisParams, error) {
	p := &AnalysisParams{
		TenantID: tenantID,
		UserID:   userID,
	}
	if p.TenantID <= 0 {
		return nil, gerror.New("请指定要分析的租户")
	}

	var err error
	if p.TimeStart, err = parseTimeBound(timeStart); err != nil {
		return nil, gerror.New("开始时间格式应为 YYYY-MM-DD HH:mm:ss 或 YYYY-MM-DD")
	}
	if p.TimeEnd, err = parseEndBound(timeEnd); err != nil {
		return nil, gerror.New("结束时间格式应为 YYYY-MM-DD HH:mm:ss 或 YYYY-MM-DD")
	}
	if !p.TimeStart.Before(p.TimeEnd) {
		return nil, gerror.New("开始时间必须早于结束时间")
	}
	if p.TimeEnd.Sub(p.TimeStart) > MaxAnalysisSpanDays*24*time.Hour {
		return nil, gerror.Newf("分析窗口一次最多 %d 天，请分段发起", MaxAnalysisSpanDays)
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
