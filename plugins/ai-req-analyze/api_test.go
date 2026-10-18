package aireqanalyze

import (
	"testing"
	"time"
)

func TestParseTimeBound(t *testing.T) {
	cases := []struct {
		in     string
		valid  bool
		expect string // 期望的格式化结果（用同一 layout 回显）
	}{
		{"2026-10-01 09:30:00", true, "2026-10-01 09:30:00"},
		{"2026-10-01 09:30", true, "2026-10-01 09:30:00"},
		{"2026-10-01", true, "2026-10-01 00:00:00"},
		{"2026/10/01", false, ""},
		{"", false, ""},
	}
	for _, c := range cases {
		got, err := parseTimeBound(c.in)
		if c.valid != (err == nil) {
			t.Fatalf("%q 有效性错误: %v", c.in, err)
		}
		if c.valid && got.Format("2006-01-02 15:04:05") != c.expect {
			t.Fatalf("%q 解析为 %q，期望 %q", c.in, got.Format("2006-01-02 15:04:05"), c.expect)
		}
	}
}

func TestParseEndBound(t *testing.T) {
	// 日期粒度补到当天 23:59:59
	got, err := parseEndBound("2026-10-01")
	if err != nil || got.Format("2006-01-02 15:04:05") != "2026-10-01 23:59:59" {
		t.Fatalf("结束日期补界错误: %v %v", got, err)
	}
	// 带时间原样解析
	got, err = parseEndBound("2026-10-01 12:00:00")
	if err != nil || got.Format("2006-01-02 15:04:05") != "2026-10-01 12:00:00" {
		t.Fatalf("结束时间解析错误: %v %v", got, err)
	}
	// 非法输入报错
	if _, err := parseEndBound("not-a-date"); err == nil {
		t.Fatal("非法输入应报错")
	}
	_ = time.Local
}

func TestJobRunnerStartStopCycle(t *testing.T) {
	// 启停循环必须可重复（插件禁用→再启用场景）；不触 DB，只验证池的启停
	for i := 0; i < 2; i++ {
		ensureJobRunner()
		if !runnerOn {
			t.Fatalf("第 %d 次启动后 runnerOn 应为 true", i+1)
		}
		stopJobRunner()
		if runnerOn {
			t.Fatalf("第 %d 次停止后 runnerOn 应为 false", i+1)
		}
	}
	// 重复停止幂等
	stopJobRunner()
}
