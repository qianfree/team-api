package billing

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/logic/billing/formula"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// TestFormulaRegisterFactKeys 动态注册的 facts 键可通过烟测校验（JS 插件 usageSchema 激活路径）。
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
