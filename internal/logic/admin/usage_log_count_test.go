package admin

import (
	"testing"
	"time"

	v1 "github.com/qianfree/team-api/api/admin/v1"
)

// tsStr 解析本地时区时间戳（与 parseUsageLogRange 同口径）
func tsStr(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		t.Fatalf("解析时间 %q 失败: %v", s, err)
	}
	return parsed
}

func tsPtr(t *testing.T, s string) *time.Time {
	t.Helper()
	v := tsStr(t, s)
	return &v
}

func dayDate(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func TestUsageLogCountDailyServable(t *testing.T) {
	base := v1.AdminUsageLogFilter{TenantID: 1, Model: "gpt-4o", ChannelID: 2, Status: "success"}
	if !usageLogCountDailyServable(base) {
		t.Fatal("仅维度条件时应当可走日汇总")
	}
	unservable := []v1.AdminUsageLogFilter{
		{ID: 100},
		{UserID: 5},
		{ApiKeyID: 7},
		{RequestType: 2},
		{TenantID: 1, UserID: 5},
	}
	for _, f := range unservable {
		if usageLogCountDailyServable(f) {
			t.Fatalf("筛选 %+v 无法映射日汇总维度，应返回 false", f)
		}
	}
}

func TestParseUsageLogRange(t *testing.T) {
	// 仅日期：起点补 00:00:00，终点补 23:59:59
	start, end, ok := parseUsageLogRange(v1.AdminUsageLogFilter{StartDate: "2026-09-12", EndDate: "2026-09-19"})
	if !ok {
		t.Fatal("合法日期应解析成功")
	}
	if !start.Equal(tsStr(t, "2026-09-12 00:00:00")) {
		t.Fatalf("起点应为当天 00:00:00，实际 %v", start)
	}
	if !end.Equal(tsStr(t, "2026-09-19 23:59:59")) {
		t.Fatalf("终点应为当天 23:59:59，实际 %v", end)
	}
	// 带时刻：原样使用
	start, end, ok = parseUsageLogRange(v1.AdminUsageLogFilter{StartDate: "2026-09-12 14:30:00", EndDate: "2026-09-18 09:15:00"})
	if !ok || !start.Equal(tsStr(t, "2026-09-12 14:30:00")) || !end.Equal(tsStr(t, "2026-09-18 09:15:00")) {
		t.Fatalf("带时刻应原样解析，实际 %v ~ %v", start, end)
	}
	// 缺省：返回 nil
	start, end, ok = parseUsageLogRange(v1.AdminUsageLogFilter{})
	if !ok || start != nil || end != nil {
		t.Fatalf("缺省时间应返回 nil/nil/true，实际 %v %v %v", start, end, ok)
	}
	// 非法格式
	if _, _, ok = parseUsageLogRange(v1.AdminUsageLogFilter{StartDate: "2026/09/12"}); ok {
		t.Fatal("非法格式应返回 ok=false")
	}
}

func TestBuildUsageLogCountPlan(t *testing.T) {
	now := tsStr(t, "2026-09-19 15:00:00") // 今天 09-19，昨天 09-18
	retention90 := 90                      // 保留期截断点 = 06-21

	t.Run("一周范围：当天落尾段，起始天整走日汇总", func(t *testing.T) {
		plan := buildUsageLogCountPlan(tsPtr(t, "2026-09-12 00:00:00"), tsPtr(t, "2026-09-19 23:59:59"), now, retention90, dayDate(2026, 6, 1))
		if plan.fallback {
			t.Fatal("一周范围不应回退")
		}
		if !plan.dailyFrom.Equal(dayDate(2026, 9, 12)) || !plan.dailyTo.Equal(dayDate(2026, 9, 18)) {
			t.Fatalf("完整天窗口应为 [09-12, 09-18]，实际 [%v, %v]", plan.dailyFrom, plan.dailyTo)
		}
		if plan.hasHead {
			t.Fatal("起点为整日 00:00 时头段应为空")
		}
		if !plan.hasTail || !plan.tailFrom.Equal(dayDate(2026, 9, 19)) || !plan.tailTo.Equal(tsStr(t, "2026-09-19 23:59:59")) {
			t.Fatalf("尾段应覆盖结束日当天，实际 from=%v to=%v", plan.tailFrom, plan.tailTo)
		}
	})

	t.Run("起点带非零时刻：起始天拆进头段开区间", func(t *testing.T) {
		plan := buildUsageLogCountPlan(tsPtr(t, "2026-09-12 14:30:00"), nil, now, retention90, dayDate(2026, 6, 1))
		if plan.fallback || !plan.dailyFrom.Equal(dayDate(2026, 9, 13)) {
			t.Fatalf("完整天应从 09-13 起，实际 dailyFrom=%v fallback=%v", plan.dailyFrom, plan.fallback)
		}
		if !plan.hasHead || !plan.headFrom.Equal(tsStr(t, "2026-09-12 14:30:00")) || !plan.headTo.Equal(dayDate(2026, 9, 13)) {
			t.Fatalf("头段应为 [09-12 14:30, 09-13 00:00)，实际 from=%v to=%v", plan.headFrom, plan.headTo)
		}
		if !plan.hasTail || !plan.tailFrom.Equal(dayDate(2026, 9, 19)) || plan.tailTo != nil {
			t.Fatalf("无结束时间时尾段应 [09-19, 无上界)，实际 from=%v to=%v", plan.tailFrom, plan.tailTo)
		}
	})

	t.Run("双侧无界：保留下界=保留期与日汇总起点的较大者", func(t *testing.T) {
		// minStat(07-01) 晚于保留期截断点(06-21)：下界取 minStat，之前落头段
		plan := buildUsageLogCountPlan(nil, nil, now, retention90, dayDate(2026, 7, 1))
		if plan.fallback || !plan.dailyFrom.Equal(dayDate(2026, 7, 1)) || !plan.dailyTo.Equal(dayDate(2026, 9, 18)) {
			t.Fatalf("完整天窗口应为 [07-01, 09-18]，实际 [%v, %v]", plan.dailyFrom, plan.dailyTo)
		}
		if !plan.hasHead || plan.headFrom != nil || !plan.headTo.Equal(dayDate(2026, 7, 1)) {
			t.Fatalf("头段应为 [无下界, 07-01)，实际 from=%v to=%v", plan.headFrom, plan.headTo)
		}
	})

	t.Run("起点早于保留期：截断点前落头段", func(t *testing.T) {
		plan := buildUsageLogCountPlan(tsPtr(t, "2026-05-01 00:00:00"), nil, now, retention90, dayDate(2026, 6, 1))
		if plan.fallback || !plan.dailyFrom.Equal(dayDate(2026, 6, 21)) {
			t.Fatalf("完整天下界应被保留期截到 06-21，实际 %v", plan.dailyFrom)
		}
		if !plan.hasHead || !plan.headFrom.Equal(dayDate(2026, 5, 1)) || !plan.headTo.Equal(dayDate(2026, 6, 21)) {
			t.Fatalf("头段应为 [05-01, 06-21)，实际 from=%v to=%v", plan.headFrom, plan.headTo)
		}
	})

	t.Run("回填缺口：minStat 晚于保留期截断点时下界取 minStat", func(t *testing.T) {
		plan := buildUsageLogCountPlan(tsPtr(t, "2026-05-01 00:00:00"), nil, now, retention90, dayDate(2026, 7, 1))
		if plan.fallback || !plan.dailyFrom.Equal(dayDate(2026, 7, 1)) {
			t.Fatalf("完整天下界应取 minStat=07-01，实际 %v", plan.dailyFrom)
		}
		if !plan.hasHead || !plan.headTo.Equal(dayDate(2026, 7, 1)) {
			t.Fatalf("缺口段应落入头段 [05-01, 07-01)，实际 to=%v", plan.headTo)
		}
	})

	t.Run("保留期关闭：不做截断", func(t *testing.T) {
		plan := buildUsageLogCountPlan(nil, nil, now, 0, dayDate(2026, 6, 1))
		if plan.fallback || !plan.dailyFrom.Equal(dayDate(2026, 6, 1)) {
			t.Fatalf("retention<=0 时下界应仅由 minStat 决定，实际 %v", plan.dailyFrom)
		}
	})

	t.Run("范围仅覆盖近两天：昨天走日汇总今天走尾段", func(t *testing.T) {
		plan := buildUsageLogCountPlan(tsPtr(t, "2026-09-18 00:00:00"), tsPtr(t, "2026-09-19 23:59:59"), now, retention90, dayDate(2026, 6, 1))
		if plan.fallback || !plan.dailyFrom.Equal(dayDate(2026, 9, 18)) || !plan.dailyTo.Equal(dayDate(2026, 9, 18)) {
			t.Fatalf("完整天窗口应为 [09-18, 09-18]，实际 [%v, %v]", plan.dailyFrom, plan.dailyTo)
		}
		if plan.hasHead || !plan.hasTail {
			t.Fatal("头段应为空、尾段应覆盖今天")
		}
	})

	t.Run("范围全在今天：无完整天，回退精确口径", func(t *testing.T) {
		plan := buildUsageLogCountPlan(tsPtr(t, "2026-09-19 00:00:00"), tsPtr(t, "2026-09-19 23:59:59"), now, retention90, dayDate(2026, 6, 1))
		if !plan.fallback {
			t.Fatal("无完整天可走日汇总时应回退")
		}
	})

	t.Run("范围全在日汇总起点之前：回退精确口径", func(t *testing.T) {
		plan := buildUsageLogCountPlan(tsPtr(t, "2026-05-01 00:00:00"), tsPtr(t, "2026-05-03 23:59:59"), now, retention90, dayDate(2026, 7, 1))
		if !plan.fallback {
			t.Fatalf("日汇总未覆盖该范围应回退，实际窗口 [%v, %v]", plan.dailyFrom, plan.dailyTo)
		}
	})
}
