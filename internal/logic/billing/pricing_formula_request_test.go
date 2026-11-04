package billing

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/logic/billing/formula"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// formulaRequestPricing 构造带请求规则的 formula 定价（组合式与分离字段同 GetModelPriceAt 组装口径）
func formulaRequestPricing(t *testing.T) *PricingResult {
	t.Helper()
	base := "p * 5 + c * 10"
	rules := `(header("tier") == "pro" ? 2 : 1) * (param("service_tier") == "priority" ? 3 : 1)`
	return &PricingResult{
		BillingMode:      BillingModeFormula,
		Formula:          EffectiveFormula(base, rules),
		FormulaBase:      base,
		FormulaRules:     rules,
		TenantMultiplier: 1.0,
		TimeMultiplier:   1.0,
		Currency:         "USD",
	}
}

// testEvalContext 单测版求值上下文：与 NewFormulaEvalContext 同构，但时区固定 UTC
// （pricingTimeLocation 依赖 sys_options 数据库，单测环境不可用）
func testEvalContext(pricing *PricingResult, headers http.Header, body []byte, at time.Time) *formula.EvalContext {
	ec := &formula.EvalContext{At: at, Location: time.UTC}
	if compiled, err := formula.Compile(pricing.Formula); err == nil {
		ec.Request = ResolveFormulaRequest(compiled, headers, body)
	}
	return ec
}

// TestComputeCostFormulaRequestRules 结算求值带请求探测：header/param 命中放大系数，
// 命中表随 breakdown 留痕
func TestComputeCostFormulaRequestRules(t *testing.T) {
	pricing := formulaRequestPricing(t)
	usage := &rcommon.Usage{PromptTokens: 1000, CompletionTokens: 200}

	headers := http.Header{}
	headers.Set("Tier", "pro") // 大小写不敏感
	body := []byte(`{"service_tier": "priority", "model": "gpt-x"}`)
	// 单测无 DB（pricingTimeLocation 的 GetOption 需要数据库），手工构造等价上下文
	ec := testEvalContext(pricing, headers, body, time.Now())

	bd := computeCostWithEval(pricing, usage.PromptTokens, usage.CompletionTokens, usage, 1, ec)
	if bd.FormulaError != "" {
		t.Fatalf("求值失败: %s", bd.FormulaError)
	}
	// 基础 = 1000*5 + 200*10 = 7000；header 命中 ×2，param 命中 ×3 → 42000
	if bd.FormulaFactor != 42000 {
		t.Fatalf("系数 = %v, want 42000", bd.FormulaFactor)
	}
	if math.Abs(bd.TotalCost-0.042) > 1e-9 {
		t.Fatalf("金额 = %v, want 0.042", bd.TotalCost)
	}
	if len(bd.RequestRules) != 2 || !bd.RequestRules[0].Matched || !bd.RequestRules[1].Matched {
		t.Fatalf("命中表异常: %+v", bd.RequestRules)
	}

	// 未命中路径：无 header、param 缺失 → 双默认 1
	ec2 := testEvalContext(pricing, nil, []byte(`{}`), time.Now())
	bd2 := computeCostWithEval(pricing, usage.PromptTokens, usage.CompletionTokens, usage, 1, ec2)
	if bd2.FormulaFactor != 7000 {
		t.Fatalf("未命中系数 = %v, want 7000", bd2.FormulaFactor)
	}
	if bd2.RequestRules[0].Matched || bd2.RequestRules[1].Matched {
		t.Fatalf("未命中标记异常: %+v", bd2.RequestRules)
	}

	// ec=nil（legacy 路径）：探针取空值，默认倍率
	bd3 := computeCost(pricing, usage.PromptTokens, usage.CompletionTokens, usage, 1)
	if bd3.FormulaFactor != 7000 {
		t.Fatalf("nil 上下文系数 = %v, want 7000", bd3.FormulaFactor)
	}
}

// TestFormulaTimeRuleBillAtConsistency 时间规则绑定 billAt：预扣（受理时刻）与结算
// （billAt 同值、实际执行更晚）结果恒一致——跨小时边界不漂移
func TestFormulaTimeRuleBillAtConsistency(t *testing.T) {
	base := "p * 5"
	rules := `hour("") >= 23 || hour("") < 8 ? 0.8 : 1`
	pricing := &PricingResult{
		BillingMode: BillingModeFormula, Formula: EffectiveFormula(base, rules),
		TenantMultiplier: 1.0, TimeMultiplier: 1.0, Currency: "USD",
	}
	shanghai, _ := time.LoadLocation("Asia/Shanghai")
	// 受理时刻 23:59（北京）——预扣与结算都用它，即便结算发生在 00:30
	billAt := time.Date(2026, 11, 4, 15, 59, 0, 0, time.UTC) // = 北京 23:59
	ec := &formula.EvalContext{At: billAt, Location: shanghai}

	bd := computeCostWithEval(pricing, 1000, 0, nil, 1, ec)
	if bd.FormulaError != "" {
		t.Fatalf("求值失败: %s", bd.FormulaError)
	}
	// 夜间档命中：1000*5*0.8 = 4000
	if bd.FormulaFactor != 4000 {
		t.Fatalf("夜间系数 = %v, want 4000", bd.FormulaFactor)
	}
	// 半小时后结算（进程时刻已过零点），同一 billAt 求值结果不变
	bdLate := computeCostWithEval(pricing, 1000, 0, nil, 1, ec)
	if bdLate.FormulaFactor != bd.FormulaFactor {
		t.Fatalf("跨零点结算漂移: %v vs %v", bdLate.FormulaFactor, bd.FormulaFactor)
	}
}

// TestTaskFormulaFreezeReplayConsistency 任务路径冻结重放：提交时注入 ratios 的
// formula_request（含 JSONB 往返）在预扣与结算两次求值中结果一致
func TestTaskFormulaFreezeReplayConsistency(t *testing.T) {
	base := "fixed(0.4) * seconds"
	rules := `param("resolution") == "1080p" ? 2 : 1`
	pricing := &PricingResult{
		BillingMode: BillingModeFormula, Formula: EffectiveFormula(base, rules),
		TenantMultiplier: 1.0, TimeMultiplier: 1.0, Currency: "USD",
	}
	compiled, err := formula.Compile(pricing.Formula)
	if err != nil {
		t.Fatalf("编译失败: %v", err)
	}
	// 提交时解析探测值并冻结（模拟 EstimateTaskCost 的注入路径）
	body := []byte(`{"resolution": "1080p", "duration": 5}`)
	ratios := map[string]any{}
	if req := ResolveFormulaRequest(compiled, nil, body); req != nil {
		freeze := req.ToMap()
		freeze["at"] = time.Now().Format(time.RFC3339Nano)
		ratios[ratioKeyFormulaRequest] = freeze
	}

	// JSONB 往返（billing_context 持久化 → 轮询结算读取）
	raw, _ := json.Marshal(ratios)
	thawed := map[string]any{}
	if err := json.Unmarshal(raw, &thawed); err != nil {
		t.Fatalf("JSONB 往返失败: %v", err)
	}

	g := GenericScheme{}
	estimate := g.EstimateTaskCost(pricing, ratios, nil)
	settled := g.SettleTaskCost(pricing, thawed, &rcommon.TaskMaterialUsage{OutputSeconds: 5})
	if !estimate.Equal(settled) {
		t.Fatalf("冻结重放不一致: estimate=%v settled=%v", estimate, settled)
	}
	// 1080p ×2：0.4 × 5s × 2 = 4（wrapSchemeCost 前的方案口径已含乘数）
	if !settled.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("金额 = %v, want 4", settled)
	}
	// ratios 中的非 float 冻结值不参与附加乘数（applyRatioMultipliers 跳过）
	if m := appliedRatioMultipliers(ratios); len(m) != 0 {
		t.Fatalf("formula_request 不应进入附加乘数: %v", m)
	}
}

// TestSnapshotFormulaRules 快照与摘要留痕：formula 节含基础公式/规则命中/探测值 vars，
// 摘要文本含「请求规则: … 命中」行
func TestSnapshotFormulaRules(t *testing.T) {
	pricing := formulaRequestPricing(t)
	usage := &rcommon.Usage{PromptTokens: 1000, CompletionTokens: 200}
	headers := http.Header{}
	headers.Set("tier", "pro")
	ec := testEvalContext(pricing, headers, []byte(`{"service_tier":"std"}`), time.Now())

	bd := computeCostWithEval(pricing, usage.PromptTokens, usage.CompletionTokens, usage, 1, ec)
	snap := GenerateBillingSnapshot(pricing, bd, usage, &SettlementResult{ActualCost: bd.TotalCost}, nil, ec)
	if snap.Formula == nil {
		t.Fatalf("formula 节缺失")
	}
	if snap.Formula.BaseExpr == "" || snap.Formula.BaseExpr == snap.Formula.Expr {
		t.Fatalf("BaseExpr 应为基础公式: %q", snap.Formula.BaseExpr)
	}
	if len(snap.Formula.Rules) != 2 || !snap.Formula.Rules[0].Matched || snap.Formula.Rules[1].Matched {
		t.Fatalf("快照命中表异常: %+v", snap.Formula.Rules)
	}
	// 探测值进 vars：header 命中 pro、param 为 std（字符串值）
	if snap.Formula.Vars["header:tier"] != "pro" {
		t.Fatalf("header 探测值缺失: %v", snap.Formula.Vars)
	}
	if snap.Formula.Vars["param:service_tier"] != "std" {
		t.Fatalf("param 探测值缺失: %v", snap.Formula.Vars)
	}

	summary := GenerateBillingSummary(context.Background(), snap)
	if !strings.Contains(summary, "请求规则:") || !strings.Contains(summary, "命中 ×2") {
		t.Fatalf("摘要缺少命中规则行:\n%s", summary)
	}
	if !strings.Contains(summary, "未命中") {
		t.Fatalf("摘要应含未命中规则:\n%s", summary)
	}
}
