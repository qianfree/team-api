package billing

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/logic/billing/formula"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// TestFormulaRegisterFactKeys 注册的 facts 键可通过烟测校验，未注册键被拒绝。
func TestFormulaRegisterFactKeys(t *testing.T) {
	formula.RegisterFactKeys(map[string]string{"js_test_clips": "测试用段数"})
	if err := formula.SmokeTest(`fixed(0.1) * u("js_test_clips")`); err != nil {
		t.Fatalf("动态注册键的公式应通过烟测: %v", err)
	}
	if err := formula.SmokeTest(`fixed(0.1) * u("js_test_never_registered")`); err == nil {
		t.Fatal("未注册键应被烟测拒绝")
	}
}

// TestTaskFormulaInputFactsMerge 任务路径 facts 贯通：估算（ratios 冻结）与结算
// （usage.Facts 覆盖）两个时点的 u() 取值。
func TestTaskFormulaInputFactsMerge(t *testing.T) {
	formula.RegisterFactKeys(map[string]string{"js_clips": "测试段数"})

	ratios := map[string]any{
		"formula_facts": map[string]any{"js_clips": 3.0}, // 提交期估算（冻结于 billing_context）
	}

	// 预扣时点：usage=nil，取冻结估算值
	in := taskFormulaInput(ratios, nil)
	if in.Facts["js_clips"] != 3.0 {
		t.Fatalf("预扣 facts 异常: %#v", in.Facts)
	}

	// 结算时点：官方计量覆盖估算
	in = taskFormulaInput(ratios, &rcommon.TaskMaterialUsage{Facts: map[string]float64{"js_clips": 5.0}})
	if in.Facts["js_clips"] != 5.0 {
		t.Fatalf("结算 facts 未覆盖: %#v", in.Facts)
	}
}

// TestFormulaHostUsageKeysRegistered JS 插件宿主固定用量词汇启动即注册（u() 可直接引用）；
// 字符串型 resolution 不进 u() 通道。
func TestFormulaHostUsageKeysRegistered(t *testing.T) {
	for _, key := range []string{"output_seconds", "output_count", "input_image_count", "input_video_seconds",
		"input_audio_seconds", "input_tokens", "output_tokens", "generate_audio"} {
		if err := formula.SmokeTest(`fixed(0.1) * u("` + key + `")`); err != nil {
			t.Fatalf("宿主用量键 %s 应已注册: %v", key, err)
		}
	}
	if err := formula.SmokeTest(`fixed(0.1) * u("resolution")`); err == nil {
		t.Fatal("resolution 为字符串维度，不应注册进 u() 通道")
	}
}

// TestMergeTaskFactsFloatMap 预扣期 ratios 内存形态为 map[string]float64 时 u() 也能取值
// （此前只认 JSONB 回读的 map[string]any，预扣期 u() 恒为 0）。
func TestMergeTaskFactsFloatMap(t *testing.T) {
	ratios := map[string]any{"formula_facts": map[string]float64{"output_seconds": 8}}
	if in := taskFormulaInput(ratios, nil); in.Facts["output_seconds"] != 8 {
		t.Fatalf("map[string]float64 形态 facts 未被读取: %#v", in.Facts)
	}
}

// TestSettleTaskCostFactsTrigger formula 结算重跑触发条件：仅有 Facts（无时长信号）
// 也应重跑公式——JS 插件按段计费（count）无秒数信号。
func TestSettleTaskCostFactsTrigger(t *testing.T) {
	formula.RegisterFactKeys(map[string]string{"js_clips2": "测试段数"})

	pricing := &PricingResult{
		BillingMode:      BillingModeFormula,
		Formula:          `fixed(0.1) * u("js_clips2")`,
		TenantMultiplier: 1,
	}
	ratios := map[string]any{
		"formula_facts": map[string]any{"js_clips2": 3.0},
	}

	// 结算：官方 2 段覆盖估算 3 段 → 0.1×2 = 0.2
	cost := GenericScheme{}.SettleTaskCost(pricing, ratios, &rcommon.TaskMaterialUsage{
		Facts: map[string]float64{"js_clips2": 2.0},
	})
	if !cost.Equal(decimal.NewFromFloat(0.2)) {
		t.Fatalf("按事实结算金额异常: %s（期望 0.2）", cost)
	}

	// 无 usage（上游未返回）→ 回退预扣口径（估算 3 段 = 0.3）
	cost = GenericScheme{}.SettleTaskCost(pricing, ratios, nil)
	if !cost.Equal(decimal.NewFromFloat(0.3)) {
		t.Fatalf("回退预扣口径异常: %s（期望 0.3）", cost)
	}
}

// TestTaskFormulaInputImageCountFromOfficialUsage 结算期 image_count 取官方张数（与
// u("output_count") 同源）：请求 4 张、实际产出 2 张时按 2 张多退少补；上游未回报张数时
// 回落提交时的请求张数。
func TestTaskFormulaInputImageCountFromOfficialUsage(t *testing.T) {
	ratios := map[string]any{
		"count":         4.0,
		"formula_facts": map[string]any{"output_count": 4.0},
	}

	// 预扣：无官方计量，取请求张数
	if in := taskFormulaInput(ratios, nil); in.ImageCount != 4 {
		t.Fatalf("预扣 image_count 应取请求张数 4，got %d", in.ImageCount)
	}
	// 结算：官方 2 张覆盖，且与 u() 通道同源
	official := &rcommon.TaskMaterialUsage{Facts: map[string]float64{"output_count": 2.0}}
	in := taskFormulaInput(ratios, official)
	if in.ImageCount != 2 {
		t.Fatalf("结算 image_count 应取官方张数 2，got %d", in.ImageCount)
	}
	if in.Facts["output_count"] != 2.0 {
		t.Fatalf("u() 通道应与 image_count 同源: %#v", in.Facts)
	}
	// 结算但官方未回报张数：回落请求张数
	noCount := &rcommon.TaskMaterialUsage{Facts: map[string]float64{"output_seconds": 5.0}}
	if in := taskFormulaInput(ratios, noCount); in.ImageCount != 4 {
		t.Fatalf("官方未回报张数时应回落请求张数 4，got %d", in.ImageCount)
	}

	// 金额：按张计费的公式写成 image_count 也随官方张数退补
	pricing := &PricingResult{BillingMode: BillingModeFormula, Formula: `fixed(0.1) * image_count`, TenantMultiplier: 1}
	if cost := (GenericScheme{}).SettleTaskCost(pricing, ratios, official); !cost.Equal(decimal.NewFromFloat(0.2)) {
		t.Fatalf("结算应按官方 2 张计 0.2，got %s", cost)
	}
}
