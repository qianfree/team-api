package billing

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

// TimeSegment 时段定价规则（mdl_pricing.time_segments JSONB 数组元素）。
// 语义：按数组顺序先命中先生效，可用来表达「促销时段压在常驻时段之上」；
// 未命中任何时段 → 乘数 1.0（默认价）。最终费用 = 各项小计 × 租户乘数 × 时段乘数。
type TimeSegment struct {
	Name       string  `json:"name"`                 // 时段名（写入计费快照，供账单解释）
	Days       []int   `json:"days,omitempty"`       // 适用星期 1=周一..7=周日；空=每天
	StartTime  string  `json:"start_time,omitempty"` // 每日开始时刻 "HH:MM"；与 EndTime 均空=全天
	EndTime    string  `json:"end_time,omitempty"`   // 每日结束时刻；End<Start 表示跨零点（如 22:00~06:00）
	ValidFrom  string  `json:"valid_from,omitempty"` // 生效起始日期 "2006-01-02"（含端点）；促销/定时调价用
	ValidTo    string  `json:"valid_to,omitempty"`   // 生效结束日期（含端点）；空=长期有效
	Multiplier float64 `json:"multiplier"`           // 价格乘数，0.5=半价；取值 (0,10]
}

const (
	// maxTimeSegments 单模型时段数量上限，防止配置爆炸与评估开销失控
	maxTimeSegments = 20
	// defaultPricingTimezone 时段定价默认时区（平台面向中国区运营）
	defaultPricingTimezone = "Asia/Shanghai"
)

// tzLocationCache 时区名 → *time.Location 缓存，避免每请求 LoadLocation
var tzLocationCache sync.Map

// resolveTimeMultiplier 按定价时刻评估时段乘数（纯函数，不触 DB/缓存）。
// 返回命中的乘数与时段名；未命中返回 (1.0, "")。
// 格式非法的时段直接跳过——数据在写入时已过 ValidateTimeSegments，此处仅防御性兜底。
func resolveTimeMultiplier(segments []TimeSegment, billAt time.Time, loc *time.Location) (float64, string) {
	for _, seg := range segments {
		if segmentMatches(seg, billAt, loc) {
			return seg.Multiplier, seg.Name
		}
	}
	return 1.0, ""
}

// segmentMatches 判断 billAt（换算到 loc 时区后）是否命中单个时段。
// 三个维度正交组合：日期边界 × 星期 × 每日时间窗。
func segmentMatches(seg TimeSegment, t time.Time, loc *time.Location) bool {
	local := t.In(loc)

	// 日期边界（含端点）：YYYY-MM-DD 按可比较整数
	if seg.ValidFrom != "" {
		from, ok := parseSegDate(seg.ValidFrom)
		if !ok || segDateNum(local) < from {
			return false
		}
	}
	if seg.ValidTo != "" {
		to, ok := parseSegDate(seg.ValidTo)
		if !ok || segDateNum(local) > to {
			return false
		}
	}

	// 星期过滤：Go 的 Weekday 周日=0，映射为 ISO 习惯的 1=周一..7=周日
	if len(seg.Days) > 0 {
		wd := int(local.Weekday())
		if wd == 0 {
			wd = 7
		}
		if !slices.Contains(seg.Days, wd) {
			return false
		}
	}

	// 每日时间窗：两项均空=全天；只填一项视为非法，跳过
	if seg.StartTime == "" && seg.EndTime == "" {
		return true
	}
	start, okS := parseSegClock(seg.StartTime)
	end, okE := parseSegClock(seg.EndTime)
	if !okS || !okE {
		return false
	}
	cur := local.Hour()*60 + local.Minute()
	if start <= end {
		// 常规窗口 [start, end)，结束时刻那一分钟起不再命中（08:00~ 表示到 07:59:59）
		return cur >= start && cur < end
	}
	// 跨零点窗口（如 22:00~06:00）：当日 >= start 或 次日凌晨 < end
	return cur >= start || cur < end
}

// parseSegClock 解析 "HH:MM" 为当日分钟数（严格两位数字，"9:00"/"24:00" 等视为非法）
func parseSegClock(s string) (int, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, false
	}
	h, errH := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// parseSegDate 解析 "2006-01-02" 为可比较整数 YYYYMMDD
func parseSegDate(s string) (int, bool) {
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return 0, false
	}
	return segDateNum(t), true
}

// segDateNum time.Time → YYYYMMDD 整数，用于日期边界比较
func segDateNum(t time.Time) int {
	return t.Year()*10000 + int(t.Month())*100 + t.Day()
}

// ValidateTimeSegments 校验时段定价配置（管理后台保存时调用，fail-fast 返回用户可读中文错误）
func ValidateTimeSegments(segments []TimeSegment) error {
	if len(segments) > maxTimeSegments {
		return gerror.Newf("时段数量不能超过 %d 条", maxTimeSegments)
	}
	for i, seg := range segments {
		label := seg.Name
		if label == "" {
			label = strconv.Itoa(i + 1)
		}
		if strings.TrimSpace(seg.Name) == "" {
			return gerror.Newf("第 %s 条时段名称不能为空", label)
		}
		if seg.Multiplier <= 0 || seg.Multiplier > 10 {
			return gerror.Newf("时段「%s」乘数必须在 (0, 10] 之间", seg.Name)
		}
		// 时间窗：两项要么都空（全天）要么都填
		if (seg.StartTime == "") != (seg.EndTime == "") {
			return gerror.Newf("时段「%s」开始与结束时间必须同时填写或同时留空", seg.Name)
		}
		if seg.StartTime != "" {
			if _, ok := parseSegClock(seg.StartTime); !ok {
				return gerror.Newf("时段「%s」开始时间格式必须为 HH:MM", seg.Name)
			}
			if _, ok := parseSegClock(seg.EndTime); !ok {
				return gerror.Newf("时段「%s」结束时间格式必须为 HH:MM", seg.Name)
			}
		}
		// 星期：1~7 且不重复
		seen := make(map[int]bool, len(seg.Days))
		for _, d := range seg.Days {
			if d < 1 || d > 7 {
				return gerror.Newf("时段「%s」适用日必须在 1（周一）~ 7（周日）之间", seg.Name)
			}
			if seen[d] {
				return gerror.Newf("时段「%s」适用日存在重复", seg.Name)
			}
			seen[d] = true
		}
		// 日期边界格式与先后
		if seg.ValidFrom != "" {
			if _, ok := parseSegDate(seg.ValidFrom); !ok {
				return gerror.Newf("时段「%s」生效起始日期格式必须为 YYYY-MM-DD", seg.Name)
			}
		}
		if seg.ValidTo != "" {
			if _, ok := parseSegDate(seg.ValidTo); !ok {
				return gerror.Newf("时段「%s」生效结束日期格式必须为 YYYY-MM-DD", seg.Name)
			}
		}
		if seg.ValidFrom != "" && seg.ValidTo != "" && seg.ValidFrom > seg.ValidTo {
			return gerror.Newf("时段「%s」生效起始日期不能晚于结束日期", seg.Name)
		}
	}
	return nil
}

// pricingTimeLocation 返回时段定价使用的时区（sys_options: pricing_time_timezone，默认 Asia/Shanghai）。
// Location 按 tz 字符串进程级缓存；解析失败兜底 time.Local（容器/主机 TZ 为 Asia/Shanghai）。
func pricingTimeLocation(ctx context.Context) *time.Location {
	tz := lcommon.Config().GetOption(ctx, "pricing_time_timezone")
	if tz == "" {
		tz = defaultPricingTimezone
	}
	if loc, ok := tzLocationCache.Load(tz); ok {
		return loc.(*time.Location)
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		g.Log().Warningf(ctx, "billing: 非法计费时区配置 %q，回退本地时区", tz)
		loc = time.Local
	}
	tzLocationCache.Store(tz, loc)
	return loc
}
