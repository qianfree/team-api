package billing

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/shopspring/decimal"

	rcommon "github.com/qianfree/team-api/relay/common"
)

// testConfigScheme 测试用方案：演示方案实现的正确形态——注册实例无状态单例，
// 费用参数从 pricing.SchemeConfig（不透明容器）读取，同名注册仅一次。
type testConfigScheme struct{}

func (testConfigScheme) Name() string { return "test:config-scheme" }

func (testConfigScheme) ValidateSchemeConfig(cfg json.RawMessage) error {
	var c struct {
		Base float64 `json:"base"`
	}
	if err := json.Unmarshal(cfg, &c); err != nil {
		return gerror.Wrap(err, "scheme_config 解析失败")
	}
	if c.Base < 0 {
		return gerror.New("base 不能为负")
	}
	return nil
}

func (testConfigScheme) EstimateTaskCost(pricing *PricingResult, _ map[string]any, _ []byte) decimal.Decimal {
	var c struct {
		Base float64 `json:"base"`
	}
	_ = json.Unmarshal(pricing.SchemeConfig, &c)
	return NewFromFloat(c.Base)
}

// SettleTaskCost 测试方案：usage 携带秒数时按秒 × base 计，否则回退预扣口径
func (testConfigScheme) SettleTaskCost(pricing *PricingResult, ratios map[string]any, usage *rcommon.TaskMaterialUsage) decimal.Decimal {
	if usage != nil && usage.OutputSeconds > 0 {
		var c struct {
			Base float64 `json:"base"`
		}
		_ = json.Unmarshal(pricing.SchemeConfig, &c)
		return NewFromFloat(c.Base * usage.OutputSeconds)
	}
	return testConfigScheme{}.EstimateTaskCost(pricing, ratios, nil)
}

// TestSchemeRegistry 注册表基础行为：generic 默认注册、空名/未注册回落、SchemeRegistered 判定。
func TestSchemeRegistry(t *testing.T) {
	if !SchemeRegistered(SchemeGeneric) {
		t.Fatal("generic scheme should be pre-registered")
	}
	if SchemeRegistered("test:not-registered") {
		t.Fatal("unregistered name should report false")
	}

	// 空名与未注册名都必须回落 generic（纯函数层防御）
	if got := LookupScheme(""); got.Name() != SchemeGeneric {
		t.Errorf("empty name should fall back to generic, got %s", got.Name())
	}
	if got := LookupScheme("no:such-scheme"); got.Name() != SchemeGeneric {
		t.Errorf("unknown name should fall back to generic, got %s", got.Name())
	}
}

// TestGenericSchemeConfigValidation generic 方案不接受非空 scheme_config
// （拦截「空方案带配置」的错位数据；null 视同未配置）。
func TestGenericSchemeConfigValidation(t *testing.T) {
	g := GenericScheme{}
	if err := g.ValidateSchemeConfig(nil); err != nil {
		t.Errorf("nil config should pass, got %v", err)
	}
	if err := g.ValidateSchemeConfig(json.RawMessage("null")); err != nil {
		t.Errorf("null config should pass, got %v", err)
	}
	if err := g.ValidateSchemeConfig(json.RawMessage(`{"x":1}`)); err == nil {
		t.Error("non-empty config should be rejected for generic scheme")
	}
}

// TestSchemeDispatch 特殊方案分发：pricing.Scheme 命中注册方案后估算走方案实现，
// 且通用层统一应用附加乘数、minCost 钳制（方案实现不得自行处理，全站口径收口）。
func TestSchemeDispatch(t *testing.T) {
	RegisterScheme(testConfigScheme{})

	pricing := &PricingResult{
		BillingMode:      BillingModeSpecial, // 方案模型的标准配对形态；分发只看 Scheme，billing_mode 是参考字段
		TenantMultiplier: 1.0,
		Scheme:           "test:config-scheme",
		SchemeConfig:     json.RawMessage(`{"base":2.0}`),
	}

	// 分发命中：方案读 scheme_config 返回 2.0，无附加乘数时原样（高于 minCost 不触发钳制）
	assertDecimal(t, estimateTaskCost(pricing, nil, nil), 2.0, "scheme dispatch base")

	// 通用层应用附加乘数：2.0 × 0.5 = 1.0（方案实现未乘，证明 applyRatioMultipliers 收口在骨架）
	assertDecimal(t, estimateTaskCost(pricing, map[string]any{"video_input": 0.5}, nil), 1.0, "scheme with ratio multiplier")

	// spec.* 事实键不参与乘法（通用层跳过）
	assertDecimal(t, estimateTaskCost(pricing, map[string]any{"spec.resolution": "720p"}, nil), 2.0, "spec facts not multiplied")

	// 通用层 minCost 钳制：方案低返回值（0.001）被钳到 0.01
	lowPricing := &PricingResult{
		TenantMultiplier: 1.0,
		Scheme:           "test:config-scheme",
		SchemeConfig:     json.RawMessage(`{"base":0.001}`),
	}
	assertDecimal(t, estimateTaskCost(lowPricing, nil, nil), 0.01, "scheme result clamped by minCost")
}

// TestSchemeDispatchGenericEquivalence 未声明 scheme 的定价走 generic 引擎，
// 结果与 billing_mode 直接计算一致（重构等价性的分发层验证，具体口径见 task_billing_test）。
func TestSchemeDispatchGenericEquivalence(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "per_request",
		PerRequestPrice:  0.05,
		TenantMultiplier: 1.0,
	}
	if got := LookupScheme(pricing.Scheme).Name(); got != SchemeGeneric {
		t.Errorf("empty scheme should resolve to generic, got %s", got)
	}
	assertDecimal(t, estimateTaskCost(pricing, nil, nil), 0.05, "generic equivalence per_request")
}

// TestSchemeRegisteredNameUniqueness 重名注册 panic（注册期快死优于带病运行）。
func TestSchemeRegisteredNameUniqueness(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("duplicate registration should panic")
		} else if !strings.Contains(fmt.Sprint(r), "duplicate scheme") {
			t.Errorf("panic message should mention duplicate scheme, got %v", r)
		}
	}()
	RegisterScheme(GenericScheme{})
}
