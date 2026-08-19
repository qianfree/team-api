package billing

import (
	"testing"
	"time"
)

// cst 测试用固定时区 UTC+8（与生产默认 Asia/Shanghai 偏移一致），避免依赖宿主机时区
var cst = time.FixedZone("CST", 8*3600)

// monday/wednesday/saturday/sunday 2026-08-17 为周一（下方有防御性断言），18 周二、19 周三、22 周六、23 周日
var (
	monday    = time.Date(2026, 8, 17, 0, 0, 0, 0, cst)
	wednesday = time.Date(2026, 8, 19, 0, 0, 0, 0, cst)
	saturday  = time.Date(2026, 8, 22, 0, 0, 0, 0, cst)
)

func TestTimePricing_FixtureWeekdaySanity(t *testing.T) {
	// 日期夹具防御性校验：若有人改动夹具日期导致星期错位，在此直接失败
	if monday.Weekday() != time.Monday || wednesday.Weekday() != time.Wednesday ||
		saturday.Weekday() != time.Saturday {
		t.Fatalf("测试日期夹具星期不正确: %v %v %v", monday.Weekday(), wednesday.Weekday(), saturday.Weekday())
	}
}

func TestResolveTimeMultiplier_FirstMatchWins(t *testing.T) {
	segments := []TimeSegment{
		{Name: "促销", Multiplier: 0.8, StartTime: "00:00", EndTime: "24:00"}, // "24:00" 非法，跳过
		{Name: "促销2", Multiplier: 0.8},
		{Name: "闲时", Multiplier: 0.5},
	}
	// 命中第二条（第一条格式非法被跳过），证明顺序先命中 + 坏格式防御
	mult, name := resolveTimeMultiplier(segments, wednesday.Add(10*time.Hour), cst)
	assertFloat(t, mult, 0.8, "mult")
	if name != "促销2" {
		t.Fatalf("expected 促销2, got %s", name)
	}
}

func TestResolveTimeMultiplier_NormalWindow(t *testing.T) {
	segments := []TimeSegment{{Name: "忙时", Days: []int{1, 2, 3, 4, 5}, StartTime: "09:00", EndTime: "18:00", Multiplier: 1.2}}

	cases := []struct {
		desc     string
		at       time.Time
		expected float64
	}{
		{"工作日窗口内 10:00", monday.Add(10 * time.Hour), 1.2},
		{"工作日开始时刻 09:00（含）", monday.Add(9 * time.Hour), 1.2},
		{"工作日结束时刻 18:00（不含）", monday.Add(18 * time.Hour), 1.0},
		{"工作日窗口前 08:59", monday.Add(8*time.Hour + 59*time.Minute), 1.0},
		{"周六不命中（days 过滤）", saturday.Add(10 * time.Hour), 1.0},
	}
	for _, c := range cases {
		mult, _ := resolveTimeMultiplier(segments, c.at, cst)
		assertFloat(t, mult, c.expected, c.desc)
	}
}

func TestResolveTimeMultiplier_CrossMidnight(t *testing.T) {
	segments := []TimeSegment{{Name: "夜段", StartTime: "22:00", EndTime: "06:00", Multiplier: 0.5}}

	cases := []struct {
		desc     string
		at       time.Time
		expected float64
	}{
		{"22:00 起始（含）", wednesday.Add(22 * time.Hour), 0.5},
		{"23:30 窗口内", wednesday.Add(23*time.Hour + 30*time.Minute), 0.5},
		{"次日 03:00 仍命中", wednesday.Add(24*time.Hour + 3*time.Hour), 0.5},
		{"06:00 结束（不含）", wednesday.Add(6 * time.Hour), 1.0},
		{"12:00 窗口外", wednesday.Add(12 * time.Hour), 1.0},
	}
	for _, c := range cases {
		mult, _ := resolveTimeMultiplier(segments, c.at, cst)
		assertFloat(t, mult, c.expected, c.desc)
	}
}

func TestResolveTimeMultiplier_AllDayAndEmpty(t *testing.T) {
	// 全天段（时间两项均空）
	segments := []TimeSegment{{Name: "周末", Days: []int{6, 7}, Multiplier: 0.8}}
	mult, name := resolveTimeMultiplier(segments, saturday.Add(15*time.Hour), cst)
	assertFloat(t, mult, 0.8, "周末全天")
	if name != "周末" {
		t.Fatalf("expected 周末, got %s", name)
	}

	// 无时段 → 默认 1.0
	mult, name = resolveTimeMultiplier(nil, wednesday, cst)
	assertFloat(t, mult, 1.0, "无时段")
	if name != "" {
		t.Fatalf("expected empty name, got %s", name)
	}
}

func TestResolveTimeMultiplier_DateBounds(t *testing.T) {
	segments := []TimeSegment{{Name: "促销", ValidFrom: "2026-08-17", ValidTo: "2026-08-19", Multiplier: 0.8}}

	cases := []struct {
		desc     string
		at       time.Time
		expected float64
	}{
		{"生效首日 00:05（含端点）", monday.Add(5 * time.Minute), 0.8},
		{"生效末日 23:59（含端点）", wednesday.Add(23*time.Hour + 59*time.Minute), 0.8},
		{"结束后次日", wednesday.Add(24 * time.Hour), 1.0},
		{"开始前一日", monday.Add(-time.Hour), 1.0},
	}
	for _, c := range cases {
		mult, _ := resolveTimeMultiplier(segments, c.at, cst)
		assertFloat(t, mult, c.expected, c.desc)
	}
}

func TestResolveTimeMultiplier_CombinedDimensions(t *testing.T) {
	// 日期边界 × 工作日 × 时间窗三维度正交组合
	segments := []TimeSegment{
		{Name: "九月促销忙时", Days: []int{1, 2, 3, 4, 5}, StartTime: "09:00", EndTime: "18:00",
			ValidFrom: "2026-09-01", ValidTo: "2026-09-30", Multiplier: 0.8},
	}
	septMonday := time.Date(2026, 9, 7, 10, 0, 0, 0, cst) // 2026-09-07 周一
	if septMonday.Weekday() != time.Monday {
		t.Fatalf("夹具 2026-09-07 应为周一, got %v", septMonday.Weekday())
	}
	mult, _ := resolveTimeMultiplier(segments, septMonday, cst)
	assertFloat(t, mult, 0.8, "促销期内工作日忙时")

	// 周六即使时间命中也不生效
	septSaturday := time.Date(2026, 9, 12, 10, 0, 0, 0, cst)
	mult, _ = resolveTimeMultiplier(segments, septSaturday, cst)
	assertFloat(t, mult, 1.0, "促销期内周六")

	// 工作日但窗口外
	mult, _ = resolveTimeMultiplier(segments, septMonday.Add(20*time.Hour), cst)
	assertFloat(t, mult, 1.0, "促销期内工作日夜里")

	// 促销期外的工作日忙时
	octMonday := time.Date(2026, 10, 5, 10, 0, 0, 0, cst)
	mult, _ = resolveTimeMultiplier(segments, octMonday, cst)
	assertFloat(t, mult, 1.0, "促销期外")
}

func TestResolveTimeMultiplier_BadFormatSkipped(t *testing.T) {
	segments := []TimeSegment{
		{Name: "坏时间", StartTime: "9:00", EndTime: "18:00", Multiplier: 0.1}, // "9:00" 缺前导零视为非法
		{Name: "坏小时", StartTime: "25:00", EndTime: "26:00", Multiplier: 0.2},
		{Name: "只填开始", StartTime: "09:00", Multiplier: 0.3},
		{Name: "兜底段", Multiplier: 0.9},
	}
	mult, name := resolveTimeMultiplier(segments, monday.Add(10*time.Hour), cst)
	assertFloat(t, mult, 0.9, "坏格式全部跳过后命中兜底段")
	if name != "兜底段" {
		t.Fatalf("expected 兜底段, got %s", name)
	}
}

func TestResolveTimeMultiplier_TimezoneConversion(t *testing.T) {
	// 时段按评估时区解释：UTC 16:00 = CST 00:00（次日），应命中 CST 00:00~08:00 的闲时段
	segments := []TimeSegment{{Name: "闲时", StartTime: "00:00", EndTime: "08:00", Multiplier: 0.5}}
	utc := time.FixedZone("UTC", 0)
	at := time.Date(2026, 8, 19, 16, 30, 0, 0, utc) // UTC 16:30 = CST 00:30
	mult, _ := resolveTimeMultiplier(segments, at, cst)
	assertFloat(t, mult, 0.5, "跨时区换算后命中")
}

func TestValidateTimeSegments(t *testing.T) {
	ok := []TimeSegment{
		{Name: "闲时", StartTime: "00:00", EndTime: "08:00", Multiplier: 0.5},
		{Name: "周末", Days: []int{6, 7}, Multiplier: 10},
		{Name: "促销", ValidFrom: "2026-09-01", ValidTo: "2026-09-30", Multiplier: 0.8},
	}
	if err := ValidateTimeSegments(ok); err != nil {
		t.Fatalf("合法配置不应报错: %v", err)
	}
	if err := ValidateTimeSegments(nil); err != nil {
		t.Fatalf("空配置不应报错: %v", err)
	}

	badCases := []struct {
		desc  string
		seg   TimeSegment
		errIs bool // 期望报错
	}{
		{"乘数为 0", TimeSegment{Name: "a", Multiplier: 0}, true},
		{"乘数超上限", TimeSegment{Name: "a", Multiplier: 10.1}, true},
		{"名称为空", TimeSegment{Name: "", Multiplier: 1}, true},
		{"只填开始时间", TimeSegment{Name: "a", StartTime: "09:00", Multiplier: 1}, true},
		{"时间格式非法", TimeSegment{Name: "a", StartTime: "9:00", EndTime: "18:00", Multiplier: 1}, true},
		{"小时越界", TimeSegment{Name: "a", StartTime: "24:00", EndTime: "25:00", Multiplier: 1}, true},
		{"星期越界", TimeSegment{Name: "a", Days: []int{0}, Multiplier: 1}, true},
		{"星期重复", TimeSegment{Name: "a", Days: []int{1, 1}, Multiplier: 1}, true},
		{"日期格式非法", TimeSegment{Name: "a", ValidFrom: "2026/09/01", Multiplier: 1}, true},
		{"起始晚于结束", TimeSegment{Name: "a", ValidFrom: "2026-09-30", ValidTo: "2026-09-01", Multiplier: 1}, true},
	}
	for _, c := range badCases {
		err := ValidateTimeSegments([]TimeSegment{c.seg})
		if c.errIs && err == nil {
			t.Fatalf("%s: 期望报错但通过", c.desc)
		}
	}

	// 数量上限
	var many []TimeSegment
	for range maxTimeSegments + 1 {
		many = append(many, TimeSegment{Name: "s", Multiplier: 1})
	}
	if err := ValidateTimeSegments(many); err == nil {
		t.Fatalf("超过 %d 条应报错", maxTimeSegments)
	}
}
