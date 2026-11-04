package billing

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	rcommon "github.com/qianfree/team-api/relay/common"
)

// TestAppliedRatioMultipliers 附加乘数提取必须与 applyRatioMultipliers 同一过滤口径：
// float 且 > 0 参与；spec.*（事实值）、duration/resolution（显式跳过）、string/bool、
// 非正值一律排除——快照列出的乘数集合与实际乘法链一致，复算算式才成立。
func TestAppliedRatioMultipliers(t *testing.T) {
	ratios := map[string]any{
		"video_input":      0.7,           // float 乘数，参与
		"quality":          3.5,           // float 乘数，参与
		"param_multiplier": 1.5,           // float 乘数，参与
		"param_matched":    "ruleA|ruleB", // string 说明，不参与
		"duration":         5.0,           // 秒数语义，显式跳过
		"resolution":       2.25,          // 乘数语义的存量键，显式跳过
		"spec.duration":    5.0,           // 事实值，跳过
		"spec.resolution":  "720p",
		"spec.has_video":   true,
		"base":             1.0,  // 乘数为 1 仍列出（实际乘过）
		"zero_ratio":       0.0,  // 非正数排除
		"negative":         -0.5, // 非正数排除
	}
	got := appliedRatioMultipliers(ratios, "duration", "resolution")
	want := map[string]float64{"video_input": 0.7, "quality": 3.5, "param_multiplier": 1.5, "base": 1.0}
	if len(got) != len(want) {
		t.Fatalf("appliedRatioMultipliers = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("appliedRatioMultipliers[%q] = %v, want %v", k, got[k], v)
		}
	}
	if appliedRatioMultipliers(nil) != nil {
		t.Error("nil ratios should return nil")
	}
	if appliedRatioMultipliers(map[string]any{"spec.duration": 5.0}) != nil {
		t.Error("fact-only ratios should return nil")
	}
}

// TestResolvePerSecondFacts 按秒要素回退链：
// 官方秒数+官方档位 → 提交时长+请求档位 → 默认时长；秒数钳制上限；
// 官方档位仅在与官方秒数同一分支内生效（与 SettleTaskCost 查价口径一致）。
func TestResolvePerSecondFacts(t *testing.T) {
	// 1. 官方计量优先：usage 秒数与档位都取实际值
	spec, secs := resolvePerSecondFacts(map[string]any{
		"spec.duration": 5.0, "spec.resolution": "480p",
	}, &rcommon.TaskMaterialUsage{OutputSeconds: 8, Resolution: "720P"})
	if spec != "720P" || secs != 8 {
		t.Errorf("usage-first facts = (%q, %v), want (720P, 8)", spec, secs)
	}

	// 2. usage 无秒数：整体回退提交时事实值（档位不单独采信 usage.Resolution）
	spec, secs = resolvePerSecondFacts(map[string]any{
		"spec.duration": 5.0, "spec.resolution": "480p",
	}, &rcommon.TaskMaterialUsage{Resolution: "1080P"})
	if spec != "480p" || secs != 5 {
		t.Errorf("fallback facts = (%q, %v), want (480p, 5)", spec, secs)
	}

	// 3. 提交时也无时长：默认 5s
	spec, secs = resolvePerSecondFacts(nil, nil)
	if spec != "" || secs != defaultTaskDurationSeconds {
		t.Errorf("default facts = (%q, %v), want (\"\", %v)", spec, secs, defaultTaskDurationSeconds)
	}

	// 4. 官方秒数同样钳制上限（防异常大值）
	_, secs = resolvePerSecondFacts(nil, &rcommon.TaskMaterialUsage{OutputSeconds: 99999})
	if secs != maxTaskDurationSeconds {
		t.Errorf("clamped seconds = %v, want %v", secs, maxTaskDurationSeconds)
	}
}

// TestCollectTaskBillingFacts 任务计费要素采集：
// 附加乘数（含 param_multiplier/param_matched）+ 按秒命中明细（官方计量优先）；
// token 模式不带 ratios 时要素为 nil（同步链路零侵扰）。
func TestCollectTaskBillingFacts(t *testing.T) {
	// per_second + ratios 附加乘数 + 官方素材计量
	pricing := &PricingResult{
		BillingMode:      "per_second",
		TenantMultiplier: 1.0,
		PerSecondPrices:  map[string]float64{"720P": 0.5, "*": 0.3},
	}
	ratios := map[string]any{
		"spec.duration":    5.0,
		"spec.resolution":  "480p",
		"video_input":      0.7,
		"param_multiplier": 1.2,
		"param_matched":    "night_batch",
	}
	facts := collectTaskBillingFacts(pricing, ratios, &rcommon.TaskMaterialUsage{OutputSeconds: 4, Resolution: "720P"})
	if facts == nil {
		t.Fatal("expected non-nil facts")
	}
	if facts.AppliedRatios["video_input"] != 0.7 || facts.AppliedRatios["param_multiplier"] != 1.2 {
		t.Errorf("AppliedRatios = %v, want video_input=0.7 param_multiplier=1.2", facts.AppliedRatios)
	}
	if _, has := facts.AppliedRatios["duration"]; has {
		t.Error("duration key must not be listed as applied multiplier")
	}
	if facts.ParamMatched != "night_batch" {
		t.Errorf("ParamMatched = %q, want night_batch", facts.ParamMatched)
	}
	if facts.PerSecond == nil {
		t.Fatal("expected per-second facts for per_second mode")
	}
	if facts.PerSecond.Resolution != "720P" || facts.PerSecond.Seconds != 4 || facts.PerSecond.UnitPrice != 0.5 {
		t.Errorf("PerSecond = %+v, want {720P 4 0.5}", facts.PerSecond)
	}

	// 特殊方案（素材计费）同样按矩阵命中记录输出生成组件要素
	special := &PricingResult{
		BillingMode:      BillingModeSpecial,
		TenantMultiplier: 1.0,
		PerSecondPrices:  map[string]float64{"768P": 0.35},
	}
	facts = collectTaskBillingFacts(special, map[string]any{"spec.resolution": "768P", "spec.duration": 6.0},
		&rcommon.TaskMaterialUsage{OutputSeconds: 8})
	if facts == nil || facts.PerSecond == nil {
		t.Fatal("expected per-second facts for special scheme")
	}
	if facts.PerSecond.Seconds != 8 || facts.PerSecond.UnitPrice != 0.35 || facts.PerSecond.Resolution != "768P" {
		t.Errorf("special PerSecond = %+v, want {768P 8 0.35}", facts.PerSecond)
	}

	// token 模式无 ratios：无附加乘数、无按秒明细 → nil
	tokenFacts := collectTaskBillingFacts(&PricingResult{BillingMode: "token"}, nil, nil)
	if tokenFacts != nil {
		t.Errorf("token mode without ratios should have nil facts, got %+v", tokenFacts)
	}

	// 按秒矩阵未参与定价（全零）：不记录按秒要素，避免误导
	// （附一个附加乘数使要素对象非 nil，专测 PerSecond 节省略）
	emptyMatrix := &PricingResult{BillingMode: "per_second", TenantMultiplier: 1.0}
	facts = collectTaskBillingFacts(emptyMatrix, map[string]any{"spec.duration": 5.0, "video_input": 0.7}, nil)
	if facts == nil {
		t.Fatal("expected non-nil facts (has applied ratio)")
	}
	if facts.PerSecond != nil {
		t.Errorf("empty matrix should omit per-second facts, got %+v", facts.PerSecond)
	}
}

// TestBuildTaskCostBreakdown_FactsCarried buildTaskCostBreakdown 把 ratios/usage 解析出的
// 要素挂到 breakdown.TaskFacts，供快照生成读取。
func TestBuildTaskCostBreakdown_FactsCarried(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_second",
		TenantMultiplier: 1.0,
		PerSecondPrices:  map[string]float64{"720p": 0.5},
	}
	bd := buildTaskCostBreakdown(context.Background(), pricing, 2.0, 0, 0,
		map[string]any{"spec.duration": 4.0, "spec.resolution": "720p", "video_input": 0.7}, nil)
	if bd.TaskFacts == nil {
		t.Fatal("expected TaskFacts on breakdown")
	}
	if bd.TaskFacts.PerSecond == nil || bd.TaskFacts.PerSecond.Seconds != 4 || bd.TaskFacts.PerSecond.UnitPrice != 0.5 {
		t.Errorf("PerSecond facts = %+v, want seconds=4 price=0.5", bd.TaskFacts.PerSecond)
	}
	if bd.TaskFacts.AppliedRatios["video_input"] != 0.7 {
		t.Errorf("AppliedRatios = %v, want video_input=0.7", bd.TaskFacts.AppliedRatios)
	}

	// pricing 为 nil（取价失败兜底路径）不带要素
	bd = buildTaskCostBreakdown(context.Background(), nil, 0.5, 0, 0, nil, nil)
	if bd.TaskFacts != nil {
		t.Errorf("nil pricing should not carry facts, got %+v", bd.TaskFacts)
	}
}

// TestGenerateBillingSnapshot_RatioMultipliersAndPerSecond 快照落附加乘数与按秒命中明细，
// JSON 往返保留（bil_usage_logs.billing_snapshot 的真实存取路径）。
func TestGenerateBillingSnapshot_RatioMultipliersAndPerSecond(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_second",
		BillingSource:    "base",
		TenantMultiplier: 0.85,
		PerSecondPrices:  map[string]float64{"720P": 0.5},
	}
	breakdown := &CostBreakdown{
		BillingMode:      "per_second",
		TenantMultiplier: 0.85,
		TotalCost:        1.19,
		TaskFacts: &TaskBillingFacts{
			AppliedRatios: map[string]float64{"video_input": 0.7},
			PerSecond:     &PerSecondFacts{Resolution: "720P", Seconds: 4, UnitPrice: 0.5},
		},
	}
	snap := GenerateBillingSnapshot(pricing, breakdown, nil, &SettlementResult{ActualCost: 1.19}, nil, nil)
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.Multipliers.RatioMultipliers["video_input"] != 0.7 {
		t.Errorf("RatioMultipliers = %v, want video_input=0.7", snap.Multipliers.RatioMultipliers)
	}
	if snap.PerSecond == nil || snap.PerSecond.Seconds != 4 || snap.PerSecond.UnitPrice != 0.5 || snap.PerSecond.Resolution != "720P" {
		t.Errorf("PerSecond = %+v, want {720P 4 0.5}", snap.PerSecond)
	}

	var parsed BillingSnapshot
	if err := json.Unmarshal([]byte(SnapshotToJSON(snap)), &parsed); err != nil {
		t.Fatalf("roundtrip unmarshal: %v", err)
	}
	if parsed.Multipliers.RatioMultipliers["video_input"] != 0.7 {
		t.Errorf("roundtrip RatioMultipliers lost: %+v", parsed.Multipliers.RatioMultipliers)
	}
	if parsed.PerSecond == nil || parsed.PerSecond.Resolution != "720P" {
		t.Errorf("roundtrip PerSecond lost: %+v", parsed.PerSecond)
	}

	// 无要素（同步链路/旧调用）快照不带新字段，结构零变化
	plain := GenerateBillingSnapshot(pricing, &CostBreakdown{TotalCost: 1.19}, nil, nil, nil, nil)
	if plain.Multipliers.RatioMultipliers != nil || plain.PerSecond != nil {
		t.Error("snapshot without TaskFacts should omit new fields")
	}
}

// TestGenerateBillingSummary_PerSecondLine 摘要按秒计费明细行：
// 实际计费秒数 × 命中单价（档位），官方秒数与请求时长不同也能对账。
func TestGenerateBillingSummary_PerSecondLine(t *testing.T) {
	snapshot := &BillingSnapshot{
		Pricing: BillingSnapshotPricing{
			BillingMode:   "per_second",
			BillingSource: "base",
		},
		Multipliers: BillingSnapshotMultipliers{TenantMultiplier: 0.85},
		PerSecond:   &BillingSnapshotPerSecond{Resolution: "720P", Seconds: 3.5, UnitPrice: 0.5},
		Settlement:  BillingSnapshotSettlement{PreDeductAmount: 2.5, ActualCost: 1.4875},
		RequestMeta: BillingSnapshotRequestMeta{RequestedModel: "wan3.0"},
	}
	text := GenerateBillingSummary(context.Background(), snapshot)
	if !strings.Contains(text, "按秒计费: 3.5 秒 × $0.500000/秒（档位 720P）") {
		t.Errorf("per-second line missing, got:\n%s", text)
	}
	if !strings.Contains(text, "租户倍率(0.85)") {
		t.Errorf("tenant multiplier line missing, got:\n%s", text)
	}

	// 无档位（矩阵通配兜底查价）不输出档位段
	snapshot.PerSecond = &BillingSnapshotPerSecond{Seconds: 4, UnitPrice: 0.3}
	text = GenerateBillingSummary(context.Background(), snapshot)
	if !strings.Contains(text, "按秒计费: 4 秒 × $0.300000/秒") {
		t.Errorf("per-second line without spec broken, got:\n%s", text)
	}
}

// TestGenerateBillingSummary_RatioMultiplierLine 倍率说明行列出全部附加乘数
// （按键排序保证文本确定性），param_multiplier 附命中规则说明。
func TestGenerateBillingSummary_RatioMultiplierLine(t *testing.T) {
	snapshot := &BillingSnapshot{
		Pricing: BillingSnapshotPricing{BillingMode: "token", BillingSource: "base"},
		Multipliers: BillingSnapshotMultipliers{
			TenantMultiplier: 1.0,
			RatioMultipliers: map[string]float64{
				"param_multiplier": 1.5,
				"video_input":      0.7,
			},
			ParamMatched: "night_batch",
		},
		TokenCosts: map[string]TokenCostDetail{
			"output": {Tokens: 10000, UnitPrice: 2.0, Multiplier: 1.05, Cost: 0.021},
		},
		Settlement:  BillingSnapshotSettlement{PreDeductAmount: 0.05, ActualCost: 0.021},
		RequestMeta: BillingSnapshotRequestMeta{RequestedModel: "doubao-video"},
	}
	text := GenerateBillingSummary(context.Background(), snapshot)
	// 键排序：param_multiplier 在 video_input 前
	if !strings.Contains(text, "已应用倍率: param_multiplier(1.5)（night_batch） × video_input(0.7)") {
		t.Errorf("ratio multiplier line broken, got:\n%s", text)
	}

	// 仅附加乘数生效（租户/时段均为 1）也必须展示倍率行
	snapshot.Multipliers.RatioMultipliers = map[string]float64{"video_input": 0.7}
	snapshot.Multipliers.ParamMatched = ""
	text = GenerateBillingSummary(context.Background(), snapshot)
	if !strings.Contains(text, "已应用倍率: video_input(0.7)") {
		t.Errorf("ratio-only multiplier line missing, got:\n%s", text)
	}
}

// TestBuildTokenCosts_TaskRatioMultiplierEquation 任务 token 路径费用含 ratios 附加乘数
// （RecalculateByTokens），行倍率必须把附加乘数并入乘数链：
// tokens/1M × unit_price × multiplier == cost 在 6 位精度内成立。
func TestBuildTokenCosts_TaskRatioMultiplierEquation(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      2.0,
		TenantMultiplier: 0.8,
		TimeMultiplier:   1.0,
	}
	// 10000 tokens × $2/1M × 0.8 × 0.7 = 0.0112
	breakdown := &CostBreakdown{
		OutputTokens: 10000,
		OutputCost:   0.0112,
		TaskFacts: &TaskBillingFacts{
			AppliedRatios: map[string]float64{"video_input": 0.7},
		},
	}
	costs := buildTokenCosts(pricing, breakdown)
	assertFloat(t, costs["output"].Multiplier, 0.56, "output multiplier (0.8 × 0.7)")
	want := float64(10000) / 1_000_000 * 2.0 * costs["output"].Multiplier
	if diff := want - costs["output"].Cost; diff > 0.000001 || diff < -0.000001 {
		t.Errorf("equation broken: 10000 × $2/1M × %s = %v != cost %v",
			formatMultiplier(costs["output"].Multiplier), want, costs["output"].Cost)
	}
}

// TestSnapshotEquation_TaskVideoEndToEnd 任务视频端到端算式一致性：
// RecalculateByTokens 的口径（含附加乘数）→ 快照 → 摘要，乘法算式两边相等。
// 定价无时段配置，billAt 零值兜底不影响结果。
func TestSnapshotEquation_TaskVideoEndToEnd(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		OutputPrice:      30.0,
		TenantMultiplier: 0.8,
		TimeMultiplier:   1.0,
	}
	ratios := map[string]any{"video_input": 0.5}
	// 与 RecalculateByTokens 同口径（该方法触 DB，此处按公式直算）：tokens/1M × 单价 × 租户 × 时段 × 附加乘数
	cost := decimal.NewFromInt(112500).Div(decimal.NewFromInt(1_000_000)).
		Mul(NewFromFloat(pricing.OutputPrice)).
		Mul(NewFromFloat(pricing.TenantMultiplier)).
		Mul(NewFromFloat(effectiveTimeMultiplier(pricing)))
	cost = applyRatioMultipliers(cost, ratios, "duration", "resolution")
	cost = RoundMoney(cost)
	breakdown := buildTaskCostBreakdown(context.Background(), pricing, InexactFloat64(cost), 112500, 50000, ratios, nil)
	snap := GenerateBillingSnapshot(pricing, breakdown, nil, &SettlementResult{ActualCost: InexactFloat64(cost)}, nil, nil)
	text := GenerateBillingSummary(context.Background(), snap)

	// 行算式：112500 × $30/1M × 0.4(=0.8×0.5) = $1.35
	if !strings.Contains(text, "输出: 112500 tokens × $30.000000/1M × 0.4 = $1.350000") {
		t.Errorf("output line equation broken, got:\n%s", text)
	}
	// 倍率说明行同时列出附加乘数
	if !strings.Contains(text, "已应用倍率: 租户倍率(0.8) × video_input(0.5)") {
		t.Errorf("applied multiplier line broken, got:\n%s", text)
	}
}

// TestAttachTaskSnapshot 快照附加（正常结算与 DuplicateSkip 重放共用同一来源）：
// 补齐快照/摘要/计费元数据；pricing 为 nil（取价失败已留痕）不附加任何字段。
// 重放路径可能触发用量日志补写，两类结果的计费信息必须同源。
func TestAttachTaskSnapshot(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_second",
		BillingSource:    "base",
		TenantMultiplier: 0.85,
		DiscountRatio:    0.85,
		PerSecondPrices:  map[string]float64{"720P": 0.5},
	}
	breakdown := buildTaskCostBreakdown(context.Background(), pricing, 1.7, 0, 0,
		map[string]any{"spec.duration": 4.0, "spec.resolution": "720P"}, nil)

	// DuplicateSkip 重放结果（此前无快照/退补差）：附加后与正常结算口径一致
	result := &rcommon.SettlementResult{
		PreDeductAmount: 2.0,
		ActualCost:      1.7,
		BaseCost:        breakdown.BaseCost,
		TotalCost:       1.7,
		RefundAmount:    0.3,
		DuplicateSkip:   true,
	}
	attachTaskSnapshot(context.Background(), result, pricing, breakdown, "wan3.0", nil)

	if result.BillingSnapshot == "" || result.BillingSnapshot == "null" {
		t.Fatalf("BillingSnapshot should be attached, got %q", result.BillingSnapshot)
	}
	if result.BillingSummary == "" {
		t.Error("BillingSummary should be attached")
	}
	if result.BillingMode != "per_second" || result.BillingSource != "base" {
		t.Errorf("billing meta = %q/%q, want per_second/base", result.BillingMode, result.BillingSource)
	}
	if result.RateMultiplier != 0.85 {
		t.Errorf("RateMultiplier = %v, want 0.85", result.RateMultiplier)
	}
	if result.DuplicateSkip != true {
		t.Error("DuplicateSkip flag must be preserved")
	}
	// 快照内容自洽：快照内 settlement 的退补差与 result 一致
	var snap BillingSnapshot
	if err := json.Unmarshal([]byte(result.BillingSnapshot), &snap); err != nil {
		t.Fatalf("snapshot unmarshal: %v", err)
	}
	if snap.Settlement.RefundAmount != 0.3 || snap.Settlement.PreDeductAmount != 2.0 {
		t.Errorf("snapshot settlement = %+v, want refund=0.3 pre=2.0", snap.Settlement)
	}
	if snap.RequestMeta.RequestedModel != "wan3.0" {
		t.Errorf("RequestedModel = %q, want wan3.0", snap.RequestMeta.RequestedModel)
	}
	if snap.PerSecond == nil || snap.PerSecond.Seconds != 4 {
		t.Errorf("PerSecond facts = %+v, want seconds=4", snap.PerSecond)
	}

	// pricing nil（取价失败）：不附加任何字段，不 panic
	bare := &rcommon.SettlementResult{ActualCost: 1.7, DuplicateSkip: true}
	attachTaskSnapshot(context.Background(), bare, nil, breakdown, "wan3.0", nil)
	if bare.BillingSnapshot != "" || bare.BillingSummary != "" || bare.BillingMode != "" {
		t.Errorf("nil pricing should attach nothing, got %+v", bare)
	}
}
