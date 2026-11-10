package billing

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/internal/logic/billing/formula"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// TestFormulaSnapshot_TierItems 档位线性计价式：快照不生成 token_costs（避免「× $0 = $0」行），
// 分项「数量 × 单价 × 乘数 = 费用」逐行成立且合计 = 实际费用；摘要不含公式原文
func TestFormulaSnapshot_TierItems(t *testing.T) {
	src := `ctx <= 128000 ? tier("≤128K", p * 2.5 + c * 10) : tier(">128K", p * 5 + c * 20)`
	pricing := &PricingResult{
		BillingMode:      BillingModeFormula,
		BillingSource:    "base",
		Formula:          src,
		FormulaBase:      src,
		TenantMultiplier: 0.8,
		TimeMultiplier:   1.0,
		Currency:         "USD",
	}
	usage := &rcommon.Usage{PromptTokens: 22000, CompletionTokens: 6000}
	bd := computeCost(pricing, 22000, 6000, usage, 1)
	if bd.FormulaError != "" {
		t.Fatalf("formula error: %s", bd.FormulaError)
	}
	snap := GenerateBillingSnapshot(pricing, bd, &SettlementResult{PreDeductAmount: 0.2, ActualCost: bd.TotalCost, RefundAmount: 0.2 - bd.TotalCost}, nil)

	if snap.TokenCosts != nil {
		t.Fatalf("formula 快照不应生成 token_costs: %+v", snap.TokenCosts)
	}
	f := snap.Formula
	if len(f.Tiers) != 1 || f.Tiers[0] != "≤128K" {
		t.Fatalf("tiers = %v", f.Tiers)
	}
	if len(f.Items) != 2 {
		t.Fatalf("items = %+v", f.Items)
	}
	sum := 0.0
	for _, it := range f.Items {
		want := it.Quantity * it.UnitPrice / 1e6 * it.Multiplier
		if math.Abs(want-it.Cost) > 1e-9 {
			t.Errorf("item 不自洽: %+v", it)
		}
		sum += it.Cost
	}
	if math.Abs(sum-bd.TotalCost) > 1e-9 {
		t.Fatalf("Σ items = %v, total = %v", sum, bd.TotalCost)
	}

	text := GenerateBillingSummary(context.Background(), snap)
	for _, want := range []string{
		"命中档位: ≤128K",
		"输入: 22000 tokens × $2.500000/1M × 0.8 = $0.044000",
		"输出: 6000 tokens × $10.000000/1M × 0.8 = $0.048000",
		"合计: ($0.044000 + $0.048000) = $0.092000",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("摘要缺少 %q:\n%s", want, text)
		}
	}
	for _, bad := range []string{"tier(", "ctx <=", "系数", "$0.000000/1M"} {
		if strings.Contains(text, bad) {
			t.Errorf("摘要不应含 %q:\n%s", bad, text)
		}
	}
}

// TestFormulaSnapshot_NonLinearFallback 非线性公式：展示变量取值 + 公式计价 × 乘数 = 公式费用
func TestFormulaSnapshot_NonLinearFallback(t *testing.T) {
	src := `min(p * 2 + c * 8, 100000)`
	pricing := &PricingResult{
		BillingMode: BillingModeFormula, BillingSource: "base",
		Formula: src, FormulaBase: src,
		TenantMultiplier: 0.5, TimeMultiplier: 1.0, Currency: "USD",
	}
	usage := &rcommon.Usage{PromptTokens: 1000, CompletionTokens: 500}
	bd := computeCost(pricing, 1000, 500, usage, 1)
	snap := GenerateBillingSnapshot(pricing, bd, &SettlementResult{ActualCost: bd.TotalCost}, nil)
	if snap.Formula.Items != nil {
		t.Fatalf("非线性公式不应拆分: %+v", snap.Formula.Items)
	}
	text := GenerateBillingSummary(context.Background(), snap)
	for _, want := range []string{
		"计费变量: 输出 500 · 输入 1000",
		"公式计价: $0.006000",
		"公式费用: $0.006000 × 0.5 = $0.003000",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("摘要缺少 %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "min(") {
		t.Errorf("摘要不应含公式原文:\n%s", text)
	}
}

// TestFormulaSnapshot_EvalErrorHidesDetail 求值失败：摘要只给结论不暴露错误细节（可能含公式片段），
// 错误原文保留在快照供管理端排查
func TestFormulaSnapshot_EvalErrorHidesDetail(t *testing.T) {
	pricing := &PricingResult{BillingMode: BillingModeFormula, Formula: "p * 2", TenantMultiplier: 1, TimeMultiplier: 1}
	bd := &CostBreakdown{BillingMode: BillingModeFormula, FormulaError: "公式求值失败: p * 2 ^ boom"}
	snap := GenerateBillingSnapshot(pricing, bd, &SettlementResult{ActualCost: 0.1}, nil)
	text := GenerateBillingSummary(context.Background(), snap)
	if !strings.Contains(text, "公式求值失败，按预扣额结算") || strings.Contains(text, "boom") {
		t.Fatalf("summary:\n%s", text)
	}
	if snap.Formula.Error == "" {
		t.Fatalf("快照应保留错误原文")
	}
}

// TestFormulaSnapshot_TaskVarsSameSource 任务路径：快照变量取自结算求值（seconds = 官方计量），
// 不再按同步 token 口径重建出 seconds=0 的矛盾快照
func TestFormulaSnapshot_TaskVarsSameSource(t *testing.T) {
	src := `tier("按秒", fixed(0.4) * seconds)`
	pricing := &PricingResult{
		BillingMode: BillingModeFormula, BillingSource: "base",
		Formula: src, FormulaBase: src,
		TenantMultiplier: 1, TimeMultiplier: 1, Currency: "USD",
	}
	ratios := map[string]any{"spec.duration": float64(5)}
	usage := &rcommon.TaskMaterialUsage{OutputSeconds: 8}
	cost := InexactFloat64(GenericScheme{}.SettleTaskCost(pricing, ratios, usage))
	bd := buildTaskCostBreakdown(context.Background(), pricing, cost, 0, 0, ratios, usage)
	snap := GenerateBillingSnapshot(pricing, bd, &SettlementResult{ActualCost: cost}, nil)

	if snap.Formula.Vars["seconds"] != float64(8) {
		t.Fatalf("vars.seconds = %v, want 8（官方计量）", snap.Formula.Vars["seconds"])
	}
	if len(snap.Formula.Items) != 1 || snap.Formula.Items[0].Quantity != 8 || snap.Formula.Items[0].UnitPrice != 0.4 {
		t.Fatalf("items = %+v", snap.Formula.Items)
	}
	text := GenerateBillingSummary(context.Background(), snap)
	if !strings.Contains(text, "计费秒数: 8 秒 × $0.400000/秒 = $3.200000") {
		t.Fatalf("summary:\n%s", text)
	}
}

// TestFormulaSnapshot_RuleDescription 账单摘要的请求规则用中文描述，不出现表达式原文；
// 无法描述的手写条件只标注序号
func TestFormulaSnapshot_RuleDescription(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	at := time.Date(2026, 11, 10, 10, 0, 0, 0, loc) // 周二 10:00
	rules := `(weekday() >= 1 && weekday() <= 5 && hour() >= 9 && hour() < 18 ? 2 : 1)
* (len(param("messages")) > 1 ? 1.5 : 1)`
	pricing := &PricingResult{
		BillingMode: BillingModeFormula, BillingSource: "base",
		FormulaBase: `tier("base", p * 1 + c * 4)`, FormulaRules: rules,
		TenantMultiplier: 1, TimeMultiplier: 1, Currency: "USD",
	}
	pricing.Formula = EffectiveFormula(pricing.FormulaBase, pricing.FormulaRules)
	ec := &formula.EvalContext{At: at, Location: loc, Request: &formula.RequestInput{Params: map[string]any{"messages": []any{1, 2}}}}
	usage := &rcommon.Usage{PromptTokens: 1000, CompletionTokens: 100}
	bd := computeCostWithEval(pricing, 1000, 100, usage, 1, ec, nil)
	if bd.FormulaError != "" {
		t.Fatalf("formula error: %s", bd.FormulaError)
	}
	snap := GenerateBillingSnapshot(pricing, bd, &SettlementResult{ActualCost: bd.TotalCost}, nil)
	text := GenerateBillingSummary(context.Background(), snap)
	for _, want := range []string{
		"请求规则: 周一至周五 且 09:00–18:00 → 命中 ×2",
		"请求规则: 第 2 条规则 → 命中 ×1.5",
		"已应用倍率: 请求规则(3)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("摘要缺少 %q:\n%s", want, text)
		}
	}
	for _, bad := range []string{"weekday(", "param(", "len("} {
		if strings.Contains(text, bad) {
			t.Errorf("摘要不应含表达式原文 %q:\n%s", bad, text)
		}
	}
}

// TestFormulaSnapshot_MinCostNote 任务最低消费钳制：实际费用高于分项合计时单独说明
func TestFormulaSnapshot_MinCostNote(t *testing.T) {
	src := `fixed(0.001)`
	pricing := &PricingResult{BillingMode: BillingModeFormula, Formula: src, FormulaBase: src, TenantMultiplier: 1, TimeMultiplier: 1}
	c, _ := formula.Compile(src)
	detail, _ := formula.RunDetail(c, formula.FormulaInput{})
	bd := &CostBreakdown{BillingMode: BillingModeFormula, FormulaFactor: detail.Factor, FormulaDetail: detail}
	snap := GenerateBillingSnapshot(pricing, bd, &SettlementResult{ActualCost: 0.01}, nil)
	text := GenerateBillingSummary(context.Background(), snap)
	if !strings.Contains(text, "按次: 1 次 × $0.001000/次 = $0.001000") ||
		!strings.Contains(text, "不足最低消费，按 $0.010000 计费") {
		t.Fatalf("summary:\n%s", text)
	}
}
