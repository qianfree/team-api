package billing

import (
	"testing"
)

func TestBuildTaskCostBreakdown_NilPricing(t *testing.T) {
	bd := buildTaskCostBreakdown(nil, 0.5, 1000, 500)
	if bd == nil {
		t.Fatal("expected non-nil breakdown")
	}
	assertFloat(t, bd.TotalCost, 0.5, "TotalCost")
	assertFloat(t, bd.BaseCost, 0.5, "BaseCost")
	if bd.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", bd.Currency)
	}
}

func TestBuildTaskCostBreakdown_PerRequestMode(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_request",
		PerRequestPrice:  0.10,
		DiscountRatio:    0.9,
		TenantMultiplier: 1.0,
		Currency:         "USD",
	}

	bd := buildTaskCostBreakdown(pricing, 0.10, 0, 0)
	assertFloat(t, bd.BaseCost, 0.10, "BaseCost")
	assertFloat(t, bd.TotalCost, 0.10, "TotalCost")
	assertFloat(t, bd.PerRequestPrice, 0.10, "PerRequestPrice")
	if bd.BillingMode != "per_request" {
		t.Errorf("BillingMode = %q, want per_request", bd.BillingMode)
	}
}

func TestBuildTaskCostBreakdown_TokenMode(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      3.0,
		TenantMultiplier: 0.8,
		DiscountRatio:    0.8,
		Currency:         "USD",
	}

	bd := buildTaskCostBreakdown(pricing, 0.024, 10000, 5000)
	if bd.OutputTokens != 10000 {
		t.Errorf("OutputTokens = %d, want 10000", bd.OutputTokens)
	}
	assertFloat(t, bd.OutputCost, 0.024, "OutputCost")
	assertFloat(t, bd.TotalCost, 0.024, "TotalCost")
	// BaseCost = actualCost / tenantMultiplier = 0.024 / 0.8 = 0.03
	assertFloat(t, bd.BaseCost, 0.03, "BaseCost")
}

func TestBuildTaskCostBreakdown_TokenMode_ZeroTenantMultiplier(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      3.0,
		TenantMultiplier: 0,
		Currency:         "USD",
	}

	bd := buildTaskCostBreakdown(pricing, 0.01, 1000, 500)
	// tenantMul == 0 → BaseCost = actualCost (no division)
	assertFloat(t, bd.BaseCost, 0.01, "BaseCost")
	assertFloat(t, bd.TotalCost, 0.01, "TotalCost")
}

func TestBuildTaskCostBreakdown_TokenMode_TotalTokens(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      5.0,
		TenantMultiplier: 1.0,
		Currency:         "USD",
	}

	bd := buildTaskCostBreakdown(pricing, 0.05, 10000, 3000)
	if bd.OutputTokens != 10000 {
		t.Errorf("OutputTokens = %d, want 10000", bd.OutputTokens)
	}
}

func TestBuildTaskCostBreakdown_CarriesPricingFields(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_request",
		PerRequestPrice:  0.50,
		DiscountRatio:    0.85,
		TenantMultiplier: 0.85,
		Currency:         "USD",
	}

	bd := buildTaskCostBreakdown(pricing, 0.50, 0, 0)
	assertFloat(t, bd.DiscountRatio, 0.85, "DiscountRatio")
	assertFloat(t, bd.TenantMultiplier, 0.85, "TenantMultiplier")
	if bd.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", bd.Currency)
	}
}

// TestBuildTaskCostBreakdown_TokenMode_ZeroTokens 覆盖图片等「token 模式但无真实 token 用量」
// 的场景：费用必须整体记为 BaseCost，OutputCost/OutputTokens 保持 0，避免快照生成
// 「0 token 却有 output 费用」的自相矛盾行（如 0 tokens × $30/1M = $3.375）。
func TestBuildTaskCostBreakdown_TokenMode_ZeroTokens(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      30.0,
		TenantMultiplier: 1.0,
		Currency:         "USD",
	}

	bd := buildTaskCostBreakdown(pricing, 3.375, 0, 0)
	if bd.OutputTokens != 0 {
		t.Errorf("OutputTokens = %d, want 0", bd.OutputTokens)
	}
	assertFloat(t, bd.OutputCost, 0, "OutputCost")
	assertFloat(t, bd.BaseCost, 3.375, "BaseCost")
	assertFloat(t, bd.TotalCost, 3.375, "TotalCost")
}

// TestEstimateTaskCost_PerRequest 按次计费直接取按次单价。
func TestEstimateTaskCost_PerRequest(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_request",
		PerRequestPrice:  0.04,
		OutputPrice:      30.0, // 即使配了 output_price 也不应走 token 估算
		TenantMultiplier: 1.0,
	}
	assertFloat(t, estimateTaskCost(pricing, nil), 0.04, "per_request cost")
}

// TestEstimateTaskCost_VideoWithDurationSignal 视频任务（ratios 携带 duration/resolution）
// 才走 10000×duration×resolution 的 token 估算：默认 5s×720p(2.25)×$30/1M = $3.375。
func TestEstimateTaskCost_VideoWithDurationSignal(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      30.0,
		TenantMultiplier: 1.0,
	}
	ratios := map[string]float64{"duration": 5, "resolution": 2.25}
	assertFloat(t, estimateTaskCost(pricing, ratios), 3.375, "video cost")
}

// TestEstimateTaskCost_ImageNoDurationSignal 图片任务（token 模式、有 output_price、但 ratios 无
// 时长信号）必须退回按次口径，**不得**凭空估出 11.25 万 token 的 $3.375 预扣。
// 未配 per_request_price 时用占位预扣 $0.1（结算再按真实 token 多退少补）。
func TestEstimateTaskCost_ImageNoDurationSignal(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      30.0,
		PerRequestPrice:  0, // 未配按次价
		TenantMultiplier: 1.0,
	}
	assertFloat(t, estimateTaskCost(pricing, nil), 0.1, "image placeholder pre-deduct")
}

// TestEstimateTaskCost_PerRequestBelowPlaceholder 已配置的真实按次单价（低于占位值）必须被尊重，
// 不得被占位预扣 $0.1 抬价——否则 per_request 图片（预扣==实收）会被超扣。
func TestEstimateTaskCost_PerRequestBelowPlaceholder(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_request",
		PerRequestPrice:  0.04, // DALL·E 类真实单价，低于 0.1 占位
		TenantMultiplier: 1.0,
	}
	assertFloat(t, estimateTaskCost(pricing, nil), 0.04, "configured per_request price respected")
}

// TestEstimateTaskCost_ImageUsesPerRequestPrice 图片模型即便 billing_mode 仍是 token，只要配了
// per_request_price，无时长信号时就按按次单价估价（而非 token 估算）。
func TestEstimateTaskCost_ImageUsesPerRequestPrice(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      30.0,
		PerRequestPrice:  0.05,
		TenantMultiplier: 1.0,
	}
	assertFloat(t, estimateTaskCost(pricing, nil), 0.05, "image per_request cost")
}

// TestEstimateTaskCost_VideoInputDiscount 附加比率（video_input 折扣）在时长估算之上叠加。
func TestEstimateTaskCost_VideoInputDiscount(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      30.0,
		TenantMultiplier: 1.0,
	}
	ratios := map[string]float64{"duration": 5, "resolution": 2.25, "video_input": 0.5}
	// 3.375 × 0.5 = 1.6875
	assertFloat(t, estimateTaskCost(pricing, ratios), 1.6875, "video discounted cost")
}

// TestEstimateTaskCost_NilPricing 定价缺失时按最低消费兜底。
func TestEstimateTaskCost_NilPricing(t *testing.T) {
	assertFloat(t, estimateTaskCost(nil, nil), 0.01, "nil pricing floor")
}

// TestHasDurationSignal 区分视频（有 duration/resolution）与图片（nil）。
func TestHasDurationSignal(t *testing.T) {
	if hasDurationSignal(nil) {
		t.Error("nil ratios should not signal duration")
	}
	if hasDurationSignal(map[string]float64{"video_input": 0.5}) {
		t.Error("video_input alone should not signal duration")
	}
	if !hasDurationSignal(map[string]float64{"duration": 5}) {
		t.Error("duration key should signal")
	}
	if !hasDurationSignal(map[string]float64{"resolution": 2.25}) {
		t.Error("resolution key should signal")
	}
}
