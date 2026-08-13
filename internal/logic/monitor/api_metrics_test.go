package monitor

import (
	"math"
	"testing"

	"github.com/qianfree/team-api/internal/logic/common"
)

// 断言 list 中某模型的字段（浮点用容差比较，派生指标均四舍五入到 2 位小数）。
func assertPerfField(t *testing.T, list []map[string]any, model, field string, wantFloat float64, wantInt int64) {
	t.Helper()
	for _, item := range list {
		if item["model_name"] != model {
			continue
		}
		switch v := item[field].(type) {
		case int64:
			if v != wantInt {
				t.Errorf("%s.%s = %d, want %d", model, field, v, wantInt)
			}
			return
		case float64:
			if math.Abs(v-wantFloat) > 1e-9 {
				t.Errorf("%s.%s = %v, want %v", model, field, v, wantFloat)
			}
			return
		case string:
			if v != model {
				t.Errorf("%s.%s = %q", model, field, v)
			}
			return
		}
	}
	t.Errorf("model %s not found in list or field %s type unexpected", model, field)
}

func TestBuildModelPerformanceList_HistoryOnly(t *testing.T) {
	rows := []modelPerfRow{
		{ModelName: "gpt-4o", RequestCount: 100, SuccessCount: 90, InputTokens: 1000, OutputTokens: 2000, TotalCost: 1.5, SumLatencyMs: 50000, SumFirstTokenMs: 8000},
		{ModelName: "claude-3-5-sonnet", RequestCount: 50, SuccessCount: 1, InputTokens: 100, OutputTokens: 50, TotalCost: 0.1, SumLatencyMs: 30000, SumFirstTokenMs: 0},
	}
	list := buildModelPerformanceList(rows, nil)

	// 默认按模型名称升序：claude-3-5-sonnet（'c'）应排在 gpt-4o（'g'）前
	_ = assertOrder(t, list, "claude-3-5-sonnet", "gpt-4o")

	assertPerfField(t, list, "gpt-4o", "request_count", 0, 100)
	assertPerfField(t, list, "gpt-4o", "success_rate", 90.0, 0)
	assertGrade(t, list, "gpt-4o", "warning") // 90% 成功率 → warning
	assertPerfField(t, list, "gpt-4o", "avg_latency_ms", 500.0, 0)
	assertPerfField(t, list, "gpt-4o", "avg_first_token_ms", 80.0, 0) // 8000/100
	assertPerfField(t, list, "gpt-4o", "tps", 40.0, 0)                // 2000 / (50000/1000)
	assertPerfField(t, list, "gpt-4o", "total_tokens", 0, 3000)
	assertPerfField(t, list, "gpt-4o", "total_cost", 1.5, 0)

	// claude 成功率 2%（1/50）→ critical
	assertPerfField(t, list, "claude-3-5-sonnet", "success_rate", 2.0, 0)
	assertGrade(t, list, "claude-3-5-sonnet", "critical")
	assertPerfField(t, list, "claude-3-5-sonnet", "avg_latency_ms", 600.0, 0)
	assertPerfField(t, list, "claude-3-5-sonnet", "tps", 1.67, 0) // 50/(30000/1000)=1.6666
}

func TestBuildModelPerformanceList_MergeToday(t *testing.T) {
	rows := []modelPerfRow{
		{ModelName: "gpt-4o", RequestCount: 100, SuccessCount: 90, InputTokens: 1000, OutputTokens: 2000, TotalCost: 1.5, SumLatencyMs: 50000, SumFirstTokenMs: 8000},
	}
	today := map[string]common.ModelPerfCounter{
		"gpt-4o":        {Req: 10, Ok: 9, Lat: 6000, Ttft: 500, TtftN: 10, Tin: 50, Tout: 150, CostMicro: 500000}, // 500000 micro = 0.5 USD
		"new-model-day": {Req: 1, Ok: 0, Lat: 8000, Ttft: 0, TtftN: 0, Tin: 100, Tout: 0, CostMicro: 12345},
	}
	list := buildModelPerformanceList(rows, today)

	// 合并模型：110 请求 / 99 成功 → 90%；lat 56000/110=509.09；cost 2.0
	assertPerfField(t, list, "gpt-4o", "request_count", 0, 110)
	assertPerfField(t, list, "gpt-4o", "success_count", 0, 99)
	assertPerfField(t, list, "gpt-4o", "success_rate", 90.0, 0)
	assertPerfField(t, list, "gpt-4o", "avg_latency_ms", 509.09, 0) // 56000/110 = 509.0909
	assertPerfField(t, list, "gpt-4o", "total_tokens", 0, 3200)     // (1000+50)+(2000+150)
	assertPerfField(t, list, "gpt-4o", "total_cost", 2.0, 0)

	// 仅当日模型
	assertPerfField(t, list, "new-model-day", "request_count", 0, 1)
	assertPerfField(t, list, "new-model-day", "total_cost", 0.012345, 0)
	assertPerfField(t, list, "new-model-day", "success_rate", 0.0, 0)
	assertPerfField(t, list, "new-model-day", "avg_latency_ms", 8000.0, 0)
	assertPerfField(t, list, "new-model-day", "tps", 0.0, 0) // 无输出 token

	// 按模型名称升序：gpt-4o（'g'）在 new-model-day（'n'）前
	_ = assertOrder(t, list, "gpt-4o", "new-model-day")
}

func TestBuildModelPerformanceList_FilterUnknownAndEmpty(t *testing.T) {
	rows := []modelPerfRow{
		{ModelName: "unknown", RequestCount: 999, SuccessCount: 0},
		{ModelName: "gpt-4o", RequestCount: 10, SuccessCount: 10, SumLatencyMs: 1000},
	}
	today := map[string]common.ModelPerfCounter{
		"":          {Req: 5},           // 空模型名越过（与写入侧 Model=="" 跳过一致）
		"skip-zero": {Req: 0, Lat: 999}, // 当日请求数为 0 不并入
	}
	list := buildModelPerformanceList(rows, today)
	if len(list) != 1 || list[0]["model_name"] != "gpt-4o" {
		t.Fatalf("expected only gpt-4o, got %v", list)
	}
	assertPerfField(t, list, "gpt-4o", "request_count", 0, 10)
}

func TestBuildModelPerformanceList_Empty(t *testing.T) {
	list := buildModelPerformanceList(nil, nil)
	if list == nil || len(list) != 0 {
		t.Fatalf("expected empty list, got %v", list)
	}
}

func assertOrder(t *testing.T, list []map[string]any, first, second string) bool {
	t.Helper()
	pos := map[string]int{}
	for i, item := range list {
		pos[item["model_name"].(string)] = i
	}
	if pos[first] > pos[second] {
		t.Errorf("expected %s before %s, got positions %v", first, second, pos)
		return false
	}
	return true
}

func assertGrade(t *testing.T, list []map[string]any, model, want string) {
	t.Helper()
	for _, item := range list {
		if item["model_name"] == model {
			if g, ok := item["grade"].(string); !ok || g != want {
				t.Errorf("%s.grade = %v, want %s", model, item["grade"], want)
			}
			return
		}
	}
	t.Errorf("model %s not found in list", model)
}
