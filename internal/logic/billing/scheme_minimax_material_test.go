package billing

import (
	"encoding/json"
	"testing"

	rcommon "github.com/qianfree/team-api/relay/common"
)

// mmTestPricing 构造 MiniMax 素材方案测试定价：
// 输出矩阵 768P=0.35 / 2K=0.6；图片 5 张免费、超出 0.20/张；输入视频按秒 0.50。
// 乘数恒 1（租户/时段折扣单测用例另行覆盖）。
// billing_mode 用 special（方案模型的标准配对形态）；分发只看 Scheme，
// per_second+scheme 的存量组合由 scheme_test 覆盖。
func mmTestPricing(t *testing.T, tenantMul float64) *PricingResult {
	t.Helper()
	return &PricingResult{
		BillingMode:      BillingModeSpecial,
		TenantMultiplier: tenantMul,
		PerSecondPrices:  map[string]float64{"768P": 0.35, "2K": 0.6},
		Scheme:           SchemeMiniMaxMaterial,
		SchemeConfig: json.RawMessage(`{
			"image":       {"free_count": 5, "price_per_unit": 0.20},
			"input_video": {"price_per_second": 0.50}
		}`),
	}
}

// TestMinimaxMaterial_Estimate 预扣口径：输出组件 + 提交可知的素材组件
// （图片精确、含输入视频按生成长度上限 15s 冻结）。
func TestMinimaxMaterial_Estimate(t *testing.T) {
	pricing := mmTestPricing(t, 1.0)

	// 纯文生视频：6s × 768P 输出价，无素材费
	assertDecimal(t, estimateTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "768P",
	}, nil), 2.1, "t2v output only")

	// 8 张图：输出 2.1 + (8-5)×0.20 = 2.7（5 张内免费见下一例）
	assertDecimal(t, estimateTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "768P", "spec.input_image_count": 8.0,
	}, nil), 2.7, "8 images beyond free quota")

	// 4 张图（免费额度内）：图费 0
	assertDecimal(t, estimateTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "768P", "spec.input_image_count": 4.0,
	}, nil), 2.1, "4 images within free quota")

	// 提交不含输入视频（has_input_video 缺省）：预扣不含输入视频费
	assertDecimal(t, estimateTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "2K", "spec.input_image_count": 8.0,
	}, nil), 6*0.6+3*0.20, "no input video at estimate")

	// 提交含输入视频（时长未知）：按生成长度上限 15s 冻结 → 2.1 + 15×0.50
	assertDecimal(t, estimateTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "768P", "spec.has_input_video": true,
	}, nil), 2.1+15*0.50, "input video frozen at generation cap")

	// has_input_video=false 显式无视频：不含输入视频费
	assertDecimal(t, estimateTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "768P", "spec.has_input_video": false,
	}, nil), 2.1, "explicit no input video")

	// adaptor 确知输入时长时（spec.input_video_seconds）：按确知时长优先于 15s 上限
	assertDecimal(t, estimateTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "2K", "spec.has_input_video": true, "spec.input_video_seconds": 8.0,
	}, nil), 6*0.6+8*0.50, "input video estimated via ratios")
}

// TestMinimaxMaterial_Settle 结算口径：官方 usage 为最终计费依据。
func TestMinimaxMaterial_Settle(t *testing.T) {
	pricing := mmTestPricing(t, 1.0)
	scheme := minimaxMaterialScheme{}

	// usage 全量（768P）：输出 6×0.35 + 图 (8-5)×0.20 + 输入视频 8×0.50 = 6.7
	cost := scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "768P", "spec.input_image_count": 8.0,
	}, &rcommon.TaskMaterialUsage{InputVideoSeconds: 8, InputImageCount: 8, OutputSeconds: 6})
	assertDecimal(t, cost, 6.7, "settle full usage 768P")

	// 2K 档：输出 6×0.6 + 输入视频 10×0.50 = 8.6（输入视频按秒单一单价，与生成分辨率无关）
	cost = scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "2K",
	}, &rcommon.TaskMaterialUsage{InputVideoSeconds: 10, OutputSeconds: 6})
	assertDecimal(t, cost, 8.6, "settle 2K with input video")

	// usage 字段部分缺失：图片回退 ratios 提交时事实值，输出回退 spec.duration；
	// 输入视频缺失且提交不含视频 → 0
	cost = scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "768P", "spec.input_image_count": 8.0,
	}, &rcommon.TaskMaterialUsage{OutputSeconds: 5})
	assertDecimal(t, cost, 5*0.35+3*0.20, "settle partial usage falls back to ratios")

	// 输入视频字段缺失但提交含输入视频：回退预扣口径（15s 上限）
	cost = scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "768P", "spec.input_image_count": 8.0, "spec.has_input_video": true,
	}, &rcommon.TaskMaterialUsage{OutputSeconds: 5})
	assertDecimal(t, cost, 5*0.35+3*0.20+15*0.50, "settle missing input seconds falls back to cap estimate")

	// usage 为 nil：整体回退预扣口径（含输入视频时同样含 15s 冻结）
	cost = scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "768P", "spec.input_image_count": 8.0,
	}, nil)
	assertDecimal(t, cost, 2.7, "settle nil usage falls back to estimate")

	cost = scheme.SettleTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "768P", "spec.has_input_video": true,
	}, nil)
	assertDecimal(t, cost, 6*0.35+15*0.50, "settle nil usage with input video falls back to cap estimate")

	// 输入视频异常大值钳制到 600s（防 usage 异常刷扣）
	cost = scheme.SettleTaskCost(pricing, map[string]any{
		"spec.resolution": "768P",
	}, &rcommon.TaskMaterialUsage{InputVideoSeconds: 99999, OutputSeconds: 1})
	assertDecimal(t, cost, 1*0.35+600*0.50, "settle clamps input video seconds")
}

// TestMinimaxMaterial_Multipliers 租户折扣作用于全部组件（输出 + 素材）。
func TestMinimaxMaterial_Multipliers(t *testing.T) {
	pricing := mmTestPricing(t, 0.5)
	cost := estimateTaskCost(pricing, map[string]any{
		"spec.duration": 6.0, "spec.resolution": "768P", "spec.input_image_count": 8.0, "spec.input_video_seconds": 4.0,
	}, nil)
	assertDecimal(t, cost, (6*0.35+3*0.20+4*0.50)*0.5, "tenant multiplier applies to all components")
}

// TestMinimaxMaterial_ValidateSchemeConfig 方案私有配置校验。
func TestMinimaxMaterial_ValidateSchemeConfig(t *testing.T) {
	scheme := minimaxMaterialScheme{}

	// 合法配置
	if err := scheme.ValidateSchemeConfig(json.RawMessage(`{"image":{"free_count":5,"price_per_unit":0.2}}`)); err != nil {
		t.Errorf("valid image config rejected: %v", err)
	}
	// 空 / null / 全空 → 拒绝
	for _, cfg := range []json.RawMessage{nil, json.RawMessage("null"), json.RawMessage(`{}`)} {
		if err := scheme.ValidateSchemeConfig(cfg); err == nil {
			t.Errorf("empty config should be rejected: %s", cfg)
		}
	}
	// 负单价 / 负免费额度
	if err := scheme.ValidateSchemeConfig(json.RawMessage(`{"image":{"price_per_unit":-1}}`)); err == nil {
		t.Error("negative image price should be rejected")
	}
	if err := scheme.ValidateSchemeConfig(json.RawMessage(`{"image":{"free_count":-1,"price_per_unit":0.2}}`)); err == nil {
		t.Error("negative free count should be rejected")
	}
	// 输入视频按秒单价：零价（配置了节但不收费）与负价拒绝
	if err := scheme.ValidateSchemeConfig(json.RawMessage(`{"input_video":{"price_per_second":0}}`)); err == nil {
		t.Error("zero input video price should be rejected")
	}
	if err := scheme.ValidateSchemeConfig(json.RawMessage(`{"input_video":{"price_per_second":-0.5}}`)); err == nil {
		t.Error("negative input video price should be rejected")
	}
	if err := scheme.ValidateSchemeConfig(json.RawMessage(`{"input_video":{"price_per_second":0.5}}`)); err != nil {
		t.Errorf("valid input video price rejected: %v", err)
	}
}

// TestMinimaxMaterial_Registered 方案已注册且可经 LookupScheme 分发。
func TestMinimaxMaterial_Registered(t *testing.T) {
	if !SchemeRegistered(SchemeMiniMaxMaterial) {
		t.Fatal("minimax material scheme should be registered")
	}
	if got := LookupScheme(SchemeMiniMaxMaterial).Name(); got != SchemeMiniMaxMaterial {
		t.Errorf("lookup returned %s", got)
	}
}
