package billing

import (
	"testing"

	rcommon "github.com/qianfree/team-api/relay/common"
)

// generic per_second 按实际输出秒数结算（方案 A）的行为测试。
// 矩阵：480P=0.02 / 720P=0.05 / 1080P=0.10，"*" 兜底 0.05（对齐 wan3.0 三档定价形态）。

// perSecondTestPricing 构造 generic per_second 测试定价（乘数恒 1，租户折扣单测另行覆盖）
func perSecondTestPricing(tenantMul float64) *PricingResult {
	return &PricingResult{
		BillingMode:      "per_second",
		TenantMultiplier: tenantMul,
		PerSecondPrices:  map[string]float64{"480P": 0.02, "720P": 0.05, "1080P": 0.10, "*": 0.05},
	}
}

// TestGenericPerSecond_SettleByActualSeconds 结算口径：上游返回实际输出秒数时按
// 「实际秒数 × 档位单价 × 租户 × 时段」结算，多退少补；usage 缺失回退预扣口径。
func TestGenericPerSecond_SettleByActualSeconds(t *testing.T) {
	pricing := perSecondTestPricing(1.0)
	scheme := GenericScheme{}

	// 实际 5.13s（上游浮点秒）× 720P 档：请求 5s 预扣与实际有零头差，按实际结算
	cost := scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 5.0, "spec.resolution": "720P",
	}, &rcommon.TaskMaterialUsage{OutputSeconds: 5.13, Resolution: "720P"})
	assertDecimal(t, cost, 5.13*0.05, "settle by actual seconds")

	// usage.Resolution 优先于 ratios 提交时档位（请求未指定档位，实际输出 480P）
	cost = scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 5.0,
	}, &rcommon.TaskMaterialUsage{OutputSeconds: 5, Resolution: "480P"})
	assertDecimal(t, cost, 5*0.02, "usage resolution wins over ratios")

	// usage.Resolution 缺失回退 ratios 提交时档位；实际秒数长于请求时补扣
	cost = scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 5.0, "spec.resolution": "1080P",
	}, &rcommon.TaskMaterialUsage{OutputSeconds: 12})
	assertDecimal(t, cost, 12*0.10, "fallback resolution, supplement longer actual")

	// 智能时长场景：预扣按上限 30s 冻结，实际生成 12s 结算退差
	cost = scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 30.0, "spec.resolution": "720P",
	}, &rcommon.TaskMaterialUsage{OutputSeconds: 12, Resolution: "720P"})
	assertDecimal(t, cost, 12*0.05, "smart duration refunds difference")

	// 异常大秒数钳制 120s（与预扣钳制同源，防 usage 异常刷扣）
	cost = scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 5.0, "spec.resolution": "720P",
	}, &rcommon.TaskMaterialUsage{OutputSeconds: 99999, Resolution: "720P"})
	assertDecimal(t, cost, 120*0.05, "clamp abnormal seconds")

	// 档位未命中矩阵 → "*" 兜底价
	cost = scheme.SettleTaskCost(pricing, map[string]any{},
		&rcommon.TaskMaterialUsage{OutputSeconds: 5, Resolution: "360P"})
	assertDecimal(t, cost, 5*0.05, "unknown resolution falls back to wildcard")

	// usage 为 nil：回退预扣口径（请求 5s × 720P）
	cost = scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 5.0, "spec.resolution": "720P",
	}, nil)
	assertDecimal(t, cost, 5*0.05, "nil usage falls back to estimate")

	// usage 存在但 OutputSeconds=0：同样回退预扣口径
	cost = scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 5.0, "spec.resolution": "720P",
	}, &rcommon.TaskMaterialUsage{Resolution: "720P"})
	assertDecimal(t, cost, 5*0.05, "zero output seconds falls back to estimate")

	// 矩阵为空/全零：落到预扣分支的占位兜底（与预扣行为一致）
	empty := perSecondTestPricing(1.0)
	empty.PerSecondPrices = nil
	cost = scheme.SettleTaskCost(empty, map[string]any{
		"spec.duration": 5.0, "spec.resolution": "720P",
	}, &rcommon.TaskMaterialUsage{OutputSeconds: 5, Resolution: "720P"})
	assertDecimal(t, cost, imagePlaceholderPreDeduct, "empty matrix falls back to placeholder")
}

// TestGenericPerSecond_SettleTenantMultiplier 租户折扣作用于实际秒数结算。
func TestGenericPerSecond_SettleTenantMultiplier(t *testing.T) {
	pricing := perSecondTestPricing(0.5)
	scheme := GenericScheme{}

	cost := scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 5.0, "spec.resolution": "720P",
	}, &rcommon.TaskMaterialUsage{OutputSeconds: 5.13, Resolution: "720P"})
	assertDecimal(t, cost, 5.13*0.05*0.5, "tenant multiplier applies to actual seconds")
}

// TestGenericPerSecond_NonPerSecondModesIgnoreUsage 回归：非 per_second 模式（token 等）
// 的结算不受 usage 影响，保持「预扣即终价」。
func TestGenericPerSecond_NonPerSecondModesIgnoreUsage(t *testing.T) {
	scheme := GenericScheme{}

	// token 模式 + usage 带秒数：忽略 usage，走预扣口径（无时长信号 → 占位兜底）
	tokenPricing := &PricingResult{BillingMode: "token", OutputPrice: 3.0, TenantMultiplier: 1.0}
	cost := scheme.SettleTaskCost(tokenPricing, map[string]any{},
		&rcommon.TaskMaterialUsage{OutputSeconds: 100})
	assertDecimal(t, cost, imagePlaceholderPreDeduct, "token mode ignores usage")
}
