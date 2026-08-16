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
		{ModelName: "gpt-4o", RequestCount: 100, SuccessCount: 90, InputTokens: 1000, OutputTokens: 2000, TotalCost: 1.5, SumLatencyMs: 50000, SumFirstTokenMs: 8000,
			CacheReadTokens: 800, CacheHitRequestCount: 80},
		{ModelName: "claude-3-5-sonnet", RequestCount: 50, SuccessCount: 1, InputTokens: 100, OutputTokens: 50, TotalCost: 0.1, SumLatencyMs: 30000, SumFirstTokenMs: 0,
			CacheCreationTokens: 200, CacheReadTokens: 700, CacheHitRequestCount: 40},
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

	// 缓存：无写入，命中率 = 800/(1000+0+800) = 44.44%；请求级 = 80/100 = 80%
	assertPerfField(t, list, "gpt-4o", "cache_read_tokens", 0, 800)
	assertPerfField(t, list, "gpt-4o", "cache_creation_tokens", 0, 0)
	assertPerfField(t, list, "gpt-4o", "cache_hit_rate", 44.44, 0)
	assertPerfField(t, list, "gpt-4o", "cache_hit_request_count", 0, 80)
	assertPerfField(t, list, "gpt-4o", "cache_hit_request_rate", 80.0, 0)

	// claude 成功率 2%（1/50）→ critical
	assertPerfField(t, list, "claude-3-5-sonnet", "success_rate", 2.0, 0)
	assertGrade(t, list, "claude-3-5-sonnet", "critical")
	assertPerfField(t, list, "claude-3-5-sonnet", "avg_latency_ms", 600.0, 0)
	assertPerfField(t, list, "claude-3-5-sonnet", "tps", 1.67, 0) // 50/(30000/1000)=1.6666

	// 缓存：命中率 = 700/(100+200+700) = 70%；请求级 = 40/50 = 80%
	assertPerfField(t, list, "claude-3-5-sonnet", "cache_hit_rate", 70.0, 0)
	assertPerfField(t, list, "claude-3-5-sonnet", "cache_hit_request_rate", 80.0, 0)
}

func TestBuildModelPerformanceList_MergeToday(t *testing.T) {
	rows := []modelPerfRow{
		{ModelName: "gpt-4o", RequestCount: 100, SuccessCount: 90, InputTokens: 1000, OutputTokens: 2000, TotalCost: 1.5, SumLatencyMs: 50000, SumFirstTokenMs: 8000,
			CacheCreationTokens: 100, CacheReadTokens: 800, CacheHitRequestCount: 70},
	}
	today := map[string]common.ModelPerfCounter{
		"gpt-4o":        {Req: 10, Ok: 9, Lat: 6000, Ttft: 500, TtftN: 10, Tin: 50, Tout: 150, CostMicro: 500000, CacheCreation: 20, CacheRead: 180, CacheHitReq: 8}, // 500000 micro = 0.5 USD
		"new-model-day": {Req: 1, Ok: 0, Lat: 8000, Ttft: 0, TtftN: 0, Tin: 100, Tout: 0, CostMicro: 12345, CacheRead: 60, CacheHitReq: 1},
	}
	list := buildModelPerformanceList(rows, today)

	// 合并模型：110 请求 / 99 成功 → 90%；lat 56000/110=509.09；cost 2.0
	assertPerfField(t, list, "gpt-4o", "request_count", 0, 110)
	assertPerfField(t, list, "gpt-4o", "success_count", 0, 99)
	assertPerfField(t, list, "gpt-4o", "success_rate", 90.0, 0)
	assertPerfField(t, list, "gpt-4o", "avg_latency_ms", 509.09, 0) // 56000/110 = 509.0909
	assertPerfField(t, list, "gpt-4o", "total_tokens", 0, 3200)     // (1000+50)+(2000+150)
	assertPerfField(t, list, "gpt-4o", "total_cost", 2.0, 0)

	// 缓存合并：read 800+180=980、creation 100+20=120、命中请求 70+8=78；
	// 命中率 = 980/((1000+50)+120+980) = 980/2150 = 45.58%；请求级 = 78/110 = 70.91%
	assertPerfField(t, list, "gpt-4o", "cache_read_tokens", 0, 980)
	assertPerfField(t, list, "gpt-4o", "cache_creation_tokens", 0, 120)
	assertPerfField(t, list, "gpt-4o", "cache_hit_rate", 45.58, 0)
	assertPerfField(t, list, "gpt-4o", "cache_hit_request_count", 0, 78)
	assertPerfField(t, list, "gpt-4o", "cache_hit_request_rate", 70.91, 0)

	// 仅当日模型
	assertPerfField(t, list, "new-model-day", "request_count", 0, 1)
	assertPerfField(t, list, "new-model-day", "total_cost", 0.012345, 0)
	assertPerfField(t, list, "new-model-day", "success_rate", 0.0, 0)
	assertPerfField(t, list, "new-model-day", "avg_latency_ms", 8000.0, 0)
	assertPerfField(t, list, "new-model-day", "tps", 0.0, 0) // 无输出 token

	// 仅当日缓存：命中率 = 60/(100+0+60) = 37.5%；请求级 = 1/1 = 100%
	assertPerfField(t, list, "new-model-day", "cache_hit_rate", 37.5, 0)
	assertPerfField(t, list, "new-model-day", "cache_hit_request_rate", 100.0, 0)

	// 按模型名称升序：gpt-4o（'g'）在 new-model-day（'n'）前
	_ = assertOrder(t, list, "gpt-4o", "new-model-day")
}

// 缓存边界：仅写入未命中（creation>0、read=0）不算命中请求；无缓存活动时分母为 0 不 panic、比率为 0。
func TestBuildModelPerformanceList_CacheEdgeCases(t *testing.T) {
	rows := []modelPerfRow{
		// 首次写入缓存，尚未命中：creation 500 / read 0 / 命中请求 0
		{ModelName: "write-only", RequestCount: 10, SuccessCount: 10, InputTokens: 500, OutputTokens: 100,
			CacheCreationTokens: 500},
		// 完全无缓存活动（embedding 类模型）
		{ModelName: "text-embedding-3", RequestCount: 20, SuccessCount: 20, InputTokens: 4000, OutputTokens: 0},
	}
	list := buildModelPerformanceList(rows, nil)

	assertPerfField(t, list, "write-only", "cache_creation_tokens", 0, 500)
	assertPerfField(t, list, "write-only", "cache_read_tokens", 0, 0)
	assertPerfField(t, list, "write-only", "cache_hit_rate", 0.0, 0)         // 0/(500+500+0)
	assertPerfField(t, list, "write-only", "cache_hit_request_count", 0, 0)  // 仅写入不计命中
	assertPerfField(t, list, "write-only", "cache_hit_request_rate", 0.0, 0) // 0/10

	assertPerfField(t, list, "text-embedding-3", "cache_hit_rate", 0.0, 0)
	assertPerfField(t, list, "text-embedding-3", "cache_hit_request_rate", 0.0, 0)
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
