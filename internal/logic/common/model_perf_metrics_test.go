package common

import "testing"

// 钥匙生成与编码的纯函数单测：保证写侧（RecordModelPerfMetrics）与读侧（GetModelPerfMetricsToday）
// 用同一套 key 规则命中同一小时桶，且模型名中的特殊字符不会破坏 key 结构。
func TestModelPerfKeys(t *testing.T) {
	if got := modelPerfModelsKey("2026-08-13"); got != "perf_models:2026-08-13" {
		t.Errorf("models key = %q", got)
	}
	// 「gpt-4o」hex = 6737502d346f
	if got := modelPerfBucketKey("6737502d346f", 1700000000); got != "perf_model:6737502d346f:1700000000" {
		t.Errorf("bucket key = %q", got)
	}
	// 模型名含冒号/斜杠（如 openai/gpt-4o:renew）：hex 编码后 key 无分隔符歧义
	if got := modelPerfBucketKey(modelPerfHex("openai/gpt-4o:renew"), 1000); got != "perf_model:"+modelPerfHex("openai/gpt-4o:renew")+":1000" {
		t.Errorf("bucket key with special model = %q", got)
	}
}

// modelPerfHex 需可逆且稳定（示例值拍死，防止意外改动破坏线上已写入的 key）。
func TestModelPerfHex(t *testing.T) {
	if got := modelPerfHex("gpt-4o"); got != "6770742d346f" {
		t.Errorf("hex(gpt-4o) = %q", got)
	}
	if got := modelPerfHex(""); got != "" {
		t.Errorf("hex(empty) = %q", got)
	}
}

// microFromUSD：USD float64 → 整数 micro，四舍五入。
func TestMicroFromUSD(t *testing.T) {
	cases := []struct {
		usd  float64
		want int64
	}{
		{0, 0},
		{0.000001, 1},          // 1 micro
		{1.5, 1500000},         // 整数美元
		{0.0000004, 0},         // <0.5 micro 舍去
		{0.0000006, 1},         // >=0.5 micro 进位
		{12.3456789, 12345679}, // 12.3456789 USD
	}
	for _, c := range cases {
		if got := microFromUSD(c.usd); got != c.want {
			t.Errorf("microFromUSD(%v) = %d, want %d", c.usd, got, c.want)
		}
	}
}

func TestBoolToInt(t *testing.T) {
	if got := boolToInt(true); got != 1 {
		t.Errorf("boolToInt(true) = %d", got)
	}
	if got := boolToInt(false); got != 0 {
		t.Errorf("boolToInt(false) = %d", got)
	}
}
