package billing

import (
	"math"
	"testing"

	rcommon "github.com/qianfree/team-api/relay/common"
)

// TestComputeCostFormula computeCost formula 分支：金额 = 系数 / 1e6 × 租户乘数 × 时段乘数
func TestComputeCostFormula(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      BillingModeFormula,
		Formula:          "p * 2.5 + c * 10 + cr * 1.25",
		TenantMultiplier: 1.0,
		TimeMultiplier:   1.0,
		Currency:         "USD",
	}
	// OpenAI 口径：prompt=30000（文本15000+缓存读8000+缓存写5000+图片1500+音频500）
	usage := &rcommon.Usage{
		PromptTokens:     30000,
		CompletionTokens: 6000,
		PromptTokensDetails: &rcommon.TokenDetails{
			CachedTokens:     8000,
			CacheWriteTokens: 5000,
			ImageTokens:      1500,
			AudioTokens:      500,
		},
		CacheIncludedInPrompt: true,
	}

	bd := computeCost(pricing, usage.PromptTokens, usage.CompletionTokens, usage, 1)
	if bd.FormulaError != "" {
		t.Fatalf("公式求值失败: %s", bd.FormulaError)
	}
	// p = 30000 - 8000(cr，已单独计价) = 22000：未引用的缓存写入 5000 留在 p 按输入价计
	// （此前被扣出 p 又不单独计价，等于免费）；img/ai 未引用同样留在 p
	// 系数 = 22000*2.5 + 6000*10 + 8000*1.25 = 55000 + 60000 + 10000 = 125000
	if bd.FormulaFactor != 125000 {
		t.Fatalf("系数异常: %v", bd.FormulaFactor)
	}
	// 金额 = 125000 / 1e6 = 0.125
	if math.Abs(bd.TotalCost-0.125) > 1e-9 {
		t.Fatalf("金额异常: %v", bd.TotalCost)
	}
	if bd.BillingMode != BillingModeFormula || bd.InputTokens != 17000 || bd.OutputTokens != 6000 {
		t.Fatalf("breakdown 元数据异常: mode=%s in=%d out=%d", bd.BillingMode, bd.InputTokens, bd.OutputTokens)
	}

	// 租户乘数 × 时段乘数外挂生效
	pricing.TenantMultiplier = 0.8
	pricing.TimeMultiplier = 1.5
	bd = computeCost(pricing, usage.PromptTokens, usage.CompletionTokens, usage, 1)
	if math.Abs(bd.TotalCost-0.125*0.8*1.5) > 1e-9 {
		t.Fatalf("乘数链金额异常: %v", bd.TotalCost)
	}
}

// TestComputeCostFormulaImageCount 图片张数放大（fixed × image_count）
func TestComputeCostFormulaImageCount(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      BillingModeFormula,
		Formula:          "fixed(0.04) * image_count",
		TenantMultiplier: 1.0,
		TimeMultiplier:   1.0,
		Currency:         "USD",
	}
	bd := computeCost(pricing, 2000, 0, nil, 4)
	if bd.FormulaError != "" {
		t.Fatalf("公式求值失败: %s", bd.FormulaError)
	}
	// fixed(0.04) × 4 = 0.16
	if math.Abs(bd.TotalCost-0.16) > 1e-9 {
		t.Fatalf("按张放大金额异常: %v", bd.TotalCost)
	}
	if bd.PerRequestCount != 4 {
		t.Fatalf("张数未透出: %d", bd.PerRequestCount)
	}
}

// TestComputeCostFormulaError 求值失败 fail-safe 信号（TotalCost 无效 + FormulaError 非空）
func TestComputeCostFormulaError(t *testing.T) {
	// 保存时烟测拦截非法公式，但 PricingResult 可能来自绕过校验的手工数据：
	// 运行时必须 fail-safe 带出 FormulaError 而非 panic / 零价放行
	pricing := &PricingResult{
		BillingMode:      BillingModeFormula,
		Formula:          "unknown_var * 5",
		TenantMultiplier: 1.0,
		Currency:         "USD",
	}
	bd := computeCost(pricing, 1000, 100, nil, 1)
	if bd.FormulaError == "" {
		t.Fatal("未知变量公式应产生 FormulaError")
	}
	if bd.TotalCost != 0 {
		t.Fatalf("求值失败时 TotalCost 应无效(0): %v", bd.TotalCost)
	}
}

// TestBuildPricingBlobFormula 保存侧 formula 校验：单行约束 + 烟测拦截
func TestBuildPricingBlobFormula(t *testing.T) {
	// 合法公式
	blob, err := BuildPricingBlob([]PricingItemInput{{
		BillingMode: BillingModeFormula,
		Formula:     "ctx <= 128000 ? p * 5 + c * 20 : p * 10 + c * 40",
	}})
	if err != nil {
		t.Fatalf("合法公式构建失败: %v", err)
	}
	if blob.Formula == nil || *blob.Formula != "ctx <= 128000 ? p * 5 + c * 20 : p * 10 + c * 40" {
		t.Fatalf("公式未落 blob: %+v", blob)
	}

	// 空公式拒绝
	if _, err := BuildPricingBlob([]PricingItemInput{{BillingMode: BillingModeFormula}}); err == nil {
		t.Fatal("空公式应被拒绝")
	}
	// 语法错误拒绝
	if _, err := BuildPricingBlob([]PricingItemInput{{BillingMode: BillingModeFormula, Formula: "p *"}}); err == nil {
		t.Fatal("语法错误公式应被拒绝")
	}
	// 负值公式拒绝（多模态向量 p>0 时为负）
	if _, err := BuildPricingBlob([]PricingItemInput{{BillingMode: BillingModeFormula, Formula: "-p * 5"}}); err == nil {
		t.Fatal("负值公式应被烟测拒绝")
	}
	// 多行拒绝
	if _, err := BuildPricingBlob([]PricingItemInput{
		{BillingMode: BillingModeFormula, Formula: "p * 5"},
		{BillingMode: BillingModeFormula, Formula: "p * 10"},
	}); err == nil {
		t.Fatal("多行 formula 定价应被拒绝")
	}
}

// TestValidatePricingConfiguredFormula formula 已配置判定
func TestValidatePricingConfiguredFormula(t *testing.T) {
	ok := &PricingResult{BillingMode: BillingModeFormula, Formula: "p * 5"}
	if err := validatePricingConfigured(ok, "m"); err != nil {
		t.Fatalf("合法公式不应报未配价: %v", err)
	}
	empty := &PricingResult{BillingMode: BillingModeFormula}
	if err := validatePricingConfigured(empty, "m"); err == nil {
		t.Fatal("空公式应按未配价拒绝（fail-closed）")
	}
	// 免费公式是显式意图，不视为未配价
	free := &PricingResult{BillingMode: BillingModeFormula, Formula: "0"}
	if err := validatePricingConfigured(free, "m"); err != nil {
		t.Fatalf("免费公式（显式 0）不应被拒: %v", err)
	}
}

// TestHasTenantCustomPriceFormula formula 模型租户行不构成一口价（折扣乘数保持生效）
func TestHasTenantCustomPriceFormula(t *testing.T) {
	customPrice := 0.5
	tm := &tenantModelRow{
		CustomInputPrice: &customPrice,
		Enabled:          true,
	}
	if hasTenantCustomPrice(tm, BillingModeFormula, nil) {
		t.Fatal("formula 模式租户自定义 token 价不应构成一口价")
	}
}

// TestTaskFormulaEstimate 任务路径公式预扣估算：
// fixed(0.4) * seconds 按 spec.duration 请求时长信号求值，ratios 乘数（quality 等）由 wrapSchemeCost 连乘
func TestTaskFormulaEstimate(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      BillingModeFormula,
		Formula:          "fixed(0.4) * seconds",
		TenantMultiplier: 1.0,
		TimeMultiplier:   1.0,
		Currency:         "USD",
	}
	// 预扣：ratios 携带 spec.duration=5 → fixed(0.4)×400000×5/1e6 = 2.0
	ratios := map[string]any{"spec.duration": 5.0, "spec.resolution": "720p"}
	cost := estimateTaskCost(pricing, ratios, nil)
	if !cost.Equal(NewFromFloat(2.0)) {
		t.Fatalf("任务公式预扣异常: %v（期望 2.0）", cost)
	}

	// adaptor 档位乘数（如 kling pro quality=3.5）经 wrapSchemeCost 连乘（公式=单价结构，ratios=档位乘数）
	costWithRatio := estimateTaskCost(pricing, map[string]any{"spec.duration": 5.0, "quality": 3.5}, nil)
	if !costWithRatio.Equal(NewFromFloat(7.0)) {
		t.Fatalf("ratios 档位乘数连乘异常: %v（期望 7.0）", costWithRatio)
	}

	// 无时长信号：默认 5 秒兜底
	costDefault := estimateTaskCost(pricing, nil, nil)
	if !costDefault.Equal(NewFromFloat(2.0)) {
		t.Fatalf("默认时长兜底异常: %v（期望 2.0=0.4×5s）", costDefault)
	}

	// 求值失败（手工改库坏公式）：占位预扣 0.1 fail-safe
	bad := &PricingResult{BillingMode: BillingModeFormula, Formula: "broken (", TenantMultiplier: 1, TimeMultiplier: 1}
	costBad := estimateTaskCost(bad, nil, nil)
	if !costBad.Equal(NewFromFloat(imagePlaceholderPreDeduct)) {
		t.Fatalf("坏公式占位预扣异常: %v", costBad)
	}
}

// TestTaskFormulaImageNoDoubleCount 图片按张公式：count 已进公式 image_count，
// wrapSchemeCost 必须 skip count 防双重放大
func TestTaskFormulaImageNoDoubleCount(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      BillingModeFormula,
		Formula:          "fixed(0.04) * image_count",
		TenantMultiplier: 1.0,
		TimeMultiplier:   1.0,
	}
	// 4 张 × 0.04 = 0.16；若 wrapSchemeCost 再乘 count 会得 0.64
	cost := estimateTaskCost(pricing, map[string]any{"count": 4.0}, nil)
	if !cost.Equal(NewFromFloat(0.16)) {
		t.Fatalf("图片按张公式双重计费异常: %v（期望 0.16）", cost)
	}
}

// TestTaskFormulaSettleByMaterials 结算按官方素材秒数重算（多退少补）
func TestTaskFormulaSettleByMaterials(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      BillingModeFormula,
		Formula:          "fixed(0.4) * seconds",
		TenantMultiplier: 1.0,
		TimeMultiplier:   1.0,
	}
	ratios := map[string]any{"spec.duration": 5.0}

	// 预扣：请求 5 秒 → 2.0
	pre := estimateTaskCost(pricing, ratios, nil)
	if !pre.Equal(NewFromFloat(2.0)) {
		t.Fatalf("预扣异常: %v", pre)
	}

	// 素材重算路径（RecalculateByMaterials → SettleTaskCost）：官方实际 8 秒 → 3.2
	scheme := GenericScheme{}
	settled := scheme.SettleTaskCost(pricing, ratios, &rcommon.TaskMaterialUsage{OutputSeconds: 8})
	if !settled.Equal(NewFromFloat(3.2)) {
		t.Fatalf("素材重算异常: %v（期望 3.2）", settled)
	}

	// 智能时长实际 2 秒 → 0.8（多退）
	settledShort := scheme.SettleTaskCost(pricing, ratios, &rcommon.TaskMaterialUsage{OutputSeconds: 2})
	if !settledShort.Equal(NewFromFloat(0.8)) {
		t.Fatalf("素材重算（短时长）异常: %v（期望 0.8）", settledShort)
	}

	// usage 缺失/无秒数信号：回退预扣口径（图片按张等无时长任务=预扣即终价）
	settledFallback := scheme.SettleTaskCost(pricing, ratios, nil)
	if !settledFallback.Equal(pre) {
		t.Fatalf("素材缺失回退异常: %v", settledFallback)
	}

	// 素材重算的外层收口：wrapSchemeCost 同样 skip count（复算等式成立）
	wrapped := wrapSchemeCost(pricing, map[string]any{"count": 4.0}, NewFromFloat(0.16))
	if !wrapped.Equal(NewFromFloat(0.16)) {
		t.Fatalf("素材重算 wrap 双重计费异常: %v", wrapped)
	}
}
