package billing

import (
	"encoding/json"
	"testing"
)

// TestBuildPricingBlob_PerSecond 按秒模式：矩阵校验 + JSON 往返（数值必须是 number 而非带引号字符串）
func TestBuildPricingBlob_PerSecond(t *testing.T) {
	blob, err := BuildPricingBlob([]PricingItemInput{{
		BillingMode:     "per_second",
		PerSecondPrices: map[string]float64{"480p": 0.25, "720p": 0.5, "*": 0.5},
	}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if blob.Unit != "second" || blob.Prices["720p"] != 0.5 {
		t.Fatalf("unexpected blob: %+v", blob)
	}

	// JSONB 契约：数值序列化为 number（decimal 直接 marshal 会变字符串，此处为 float64）
	raw, _ := json.Marshal(blob)
	if !json.Valid(raw) {
		t.Fatalf("invalid json: %s", raw)
	}
	var back struct {
		Unit   string             `json:"unit"`
		Prices map[string]float64 `json:"prices"`
	}
	if err := json.Unmarshal(raw, &back); err != nil || back.Prices["720p"] != 0.5 {
		t.Fatalf("json roundtrip failed: %v %v", err, back)
	}
}

func TestBuildPricingBlob_PerSecondValidation(t *testing.T) {
	// 多行拒绝
	if _, err := BuildPricingBlob([]PricingItemInput{
		{BillingMode: "per_second", PerSecondPrices: map[string]float64{"*": 0.5}},
		{BillingMode: "per_second", MinTokens: 100, PerSecondPrices: map[string]float64{"*": 0.5}},
	}); err == nil {
		t.Fatal("expected multi-row rejection")
	}
	// 空矩阵拒绝
	if _, err := BuildPricingBlob([]PricingItemInput{{BillingMode: "per_second"}}); err == nil {
		t.Fatal("expected empty matrix rejection")
	}
	// 全零矩阵拒绝
	if _, err := BuildPricingBlob([]PricingItemInput{
		{BillingMode: "per_second", PerSecondPrices: map[string]float64{"720p": 0}},
	}); err == nil {
		t.Fatal("expected all-zero matrix rejection")
	}
	// 负价拒绝
	if _, err := BuildPricingBlob([]PricingItemInput{
		{BillingMode: "per_second", PerSecondPrices: map[string]float64{"720p": -0.1, "*": 0.5}},
	}); err == nil {
		t.Fatal("expected negative price rejection")
	}
	// 空规格键拒绝
	if _, err := BuildPricingBlob([]PricingItemInput{
		{BillingMode: "per_second", PerSecondPrices: map[string]float64{"": 0.5}},
	}); err == nil {
		t.Fatal("expected empty spec key rejection")
	}
}

// TestBuildPricingBlob_TokenAndTiered token/tiered 模式构造与阶梯数组
func TestBuildPricingBlob_TokenAndTiered(t *testing.T) {
	blob, err := BuildPricingBlob([]PricingItemInput{{
		BillingMode:    "token",
		InputPrice:     15,
		OutputPrice:    75,
		CacheReadPrice: 1.5,
	}})
	if err != nil || blob.InputPrice == nil || *blob.InputPrice != 15 {
		t.Fatalf("token blob: %+v err=%v", blob, err)
	}

	max := int64(100000)
	blob, err = BuildPricingBlob([]PricingItemInput{
		{BillingMode: "tiered", MinTokens: 0, MaxTokens: &max, InputPrice: 15, OutputPrice: 75, CacheReadPrice: 1.5},
		{BillingMode: "tiered", MinTokens: 100000, InputPrice: 12, OutputPrice: 60},
	})
	if err != nil || len(blob.Tiers) != 2 {
		t.Fatalf("tiered blob: %+v err=%v", blob, err)
	}
	if blob.Tiers[1].MaxTokens != nil {
		t.Fatalf("last tier max_tokens should be null: %+v", blob.Tiers[1])
	}
}

// TestBuildPricingBlob_ZeroPricesOmitted 0 值字段省略（omitempty）：JSON 中无该键，
// 加载侧旧列兜底语义对齐（0 价模型读 JSON 得 0，与旧行为一致）
func TestBuildPricingBlob_ZeroPricesOmitted(t *testing.T) {
	blob, err := BuildPricingBlob([]PricingItemInput{{BillingMode: "token"}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, _ := json.Marshal(blob)
	if string(raw) != "{}" {
		t.Fatalf("expected empty blob json, got %s", raw)
	}
}
