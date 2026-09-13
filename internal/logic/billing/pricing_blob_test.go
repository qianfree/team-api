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

// TestBuildPricingBlob_Special 特殊计费模式：与 per_second 同构的矩阵校验
// （special 是方案模型的 billing_mode 标识，定价形态仍是按秒矩阵）
func TestBuildPricingBlob_Special(t *testing.T) {
	// 正向：矩阵照常构造
	blob, err := BuildPricingBlob([]PricingItemInput{{
		BillingMode:     BillingModeSpecial,
		PerSecondPrices: map[string]float64{"768P": 0.35, "2K": 0.6, "*": 0.35},
	}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if blob.Unit != "second" || blob.Prices["2K"] != 0.6 {
		t.Fatalf("unexpected blob: %+v", blob)
	}

	// 多行拒绝
	if _, err := BuildPricingBlob([]PricingItemInput{
		{BillingMode: BillingModeSpecial, PerSecondPrices: map[string]float64{"*": 0.5}},
		{BillingMode: BillingModeSpecial, MinTokens: 100, PerSecondPrices: map[string]float64{"*": 0.5}},
	}); err == nil {
		t.Fatal("expected multi-row rejection")
	}
	// min_tokens ≠ 0 拒绝
	if _, err := BuildPricingBlob([]PricingItemInput{
		{BillingMode: BillingModeSpecial, MinTokens: 100, PerSecondPrices: map[string]float64{"*": 0.5}},
	}); err == nil {
		t.Fatal("expected min_tokens rejection")
	}
	// 空矩阵拒绝
	if _, err := BuildPricingBlob([]PricingItemInput{{BillingMode: BillingModeSpecial}}); err == nil {
		t.Fatal("expected empty matrix rejection")
	}
	// 全零/负价拒绝
	if _, err := BuildPricingBlob([]PricingItemInput{
		{BillingMode: BillingModeSpecial, PerSecondPrices: map[string]float64{"768P": -0.1}},
	}); err == nil {
		t.Fatal("expected non-positive matrix rejection")
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

// TestBuildPricingBlob_TieredPerTierCachePrices 阶梯逐档缓存价：档内键随行落库，
// 顶层锚点缓存价照旧写入（计费引擎的缓存 token 计费读顶层，不读档内字段）。
// JSONB 往返后两处都完整保留；旧格式（档内无缓存键）unmarshal 为 nil 不报错。
func TestBuildPricingBlob_TieredPerTierCachePrices(t *testing.T) {
	max := int64(100000)
	blob, err := BuildPricingBlob([]PricingItemInput{
		{BillingMode: "tiered", MinTokens: 0, MaxTokens: &max, InputPrice: 15, OutputPrice: 75, CacheReadPrice: 1.5, CacheCreationPrice: 18.75},
		{BillingMode: "tiered", MinTokens: 100000, InputPrice: 12, OutputPrice: 60, CacheReadPrice: 1.2, CacheCreationPrice: 15},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// 顶层锚点缓存价保留（计费引擎读取口径）
	if blob.CacheReadPrice == nil || *blob.CacheReadPrice != 1.5 || blob.CacheCreationPrice == nil || *blob.CacheCreationPrice != 18.75 {
		t.Fatalf("anchor cache prices lost: %+v", blob)
	}
	// 逐档缓存价落行
	if blob.Tiers[0].CacheReadPrice == nil || *blob.Tiers[0].CacheReadPrice != 1.5 {
		t.Fatalf("tier0 cache_read_price lost: %+v", blob.Tiers[0])
	}
	if blob.Tiers[1].CacheReadPrice == nil || *blob.Tiers[1].CacheReadPrice != 1.2 {
		t.Fatalf("tier1 cache_read_price lost: %+v", blob.Tiers[1])
	}

	// JSONB 往返（mdl_pricing.pricing 的真实存取路径）
	raw, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed := ParsePricingBlob(string(raw))
	if parsed == nil {
		t.Fatal("parse failed")
	}
	if parsed.CacheReadPrice == nil || *parsed.CacheReadPrice != 1.5 {
		t.Fatalf("roundtrip anchor cache lost: %+v", parsed)
	}
	if parsed.Tiers[1].CacheCreationPrice == nil || *parsed.Tiers[1].CacheCreationPrice != 15 {
		t.Fatalf("roundtrip tier1 cache_creation lost: %+v", parsed.Tiers[1])
	}

	// 旧格式（档内无缓存键、顶层有锚点）：unmarshal 不报错，档内为 nil
	legacy := ParsePricingBlob(`{"tiers":[{"min_tokens":0,"input_price":15,"output_price":75}],"cache_read_price":1.5}`)
	if legacy == nil || legacy.Tiers[0].CacheReadPrice != nil || legacy.CacheReadPrice == nil || *legacy.CacheReadPrice != 1.5 {
		t.Fatalf("legacy blob parse broken: %+v", legacy)
	}
}
