package formula

import (
	"math"
	"strings"
	"testing"
	"time"
)

// TestTierMatchAndItems 三目档位链：命中档位名回填，线性档位拆出分项且与系数自洽
func TestTierMatchAndItems(t *testing.T) {
	c, err := Compile(`ctx <= 128000 ? tier("≤128K", p * 2.5 + c * 10 + cr * 1.25) : tier(">128K", p * 5 + c * 20 + cr * 2.5)`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(c.Tiers) != 2 || c.Tiers[0].Name != "≤128K" || c.Tiers[1].Name != ">128K" {
		t.Fatalf("tiers = %+v", c.Tiers)
	}

	res, err := RunDetail(c, FormulaInput{BaseInputTokens: 22000, OutputTokens: 6000, CacheReadTokens: 8000})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Tiers) != 1 || res.Tiers[0] != "≤128K" {
		t.Fatalf("hit tiers = %v", res.Tiers)
	}
	// 22000*2.5 + 6000*10 + 8000*1.25 = 55000 + 60000 + 10000
	if res.Factor != 125000 {
		t.Fatalf("factor = %v", res.Factor)
	}
	if len(res.Items) != 3 || res.RuleMultiplier != 1 {
		t.Fatalf("items = %+v ruleMul = %v", res.Items, res.RuleMultiplier)
	}
	if res.Items[0].Var != "p" || res.Items[0].Quantity != 22000 || res.Items[0].Coef != 2.5 {
		t.Fatalf("item[0] = %+v", res.Items[0])
	}
	if res.Vars["ctx"] != float64(30000) {
		t.Fatalf("vars = %+v", res.Vars)
	}

	res, _ = RunDetail(c, FormulaInput{BaseInputTokens: 200000, OutputTokens: 1000})
	if len(res.Tiers) != 1 || res.Tiers[0] != ">128K" {
		t.Fatalf("long context hit = %v", res.Tiers)
	}
	// cr=0 的项不进入明细
	if len(res.Items) != 2 {
		t.Fatalf("zero quantity item not dropped: %+v", res.Items)
	}
}

// TestTierWithRequestRule 档位 × 外层请求规则：规则命中时分项 × 规则乘数 = 系数
func TestTierWithRequestRule(t *testing.T) {
	c, err := Compile(`(tier("标准", p * 2 + c * 8)) * (header("x-priority") == "high" ? 1.5 : 1)`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	in := FormulaInput{BaseInputTokens: 1000, OutputTokens: 500,
		Eval: &EvalContext{Request: &RequestInput{Headers: map[string]string{"x-priority": "high"}}}}
	res, err := RunDetail(c, in)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Factor != (1000*2+500*8)*1.5 {
		t.Fatalf("factor = %v", res.Factor)
	}
	if len(res.Items) != 2 || res.RuleMultiplier != 1.5 {
		t.Fatalf("items = %+v ruleMul = %v", res.Items, res.RuleMultiplier)
	}
}

// TestRootLinearWithoutTier 无 tier() 的线性公式（含外层规则因子）同样可拆分；
// 无 tier() 的三目公式不可拆分
func TestRootLinearWithoutTier(t *testing.T) {
	c, err := Compile(`(p * 5 + c * 20 + fixed(0.001)) * (param("service_tier") == "priority" ? 2 : 1)`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	res, err := RunDetail(c, FormulaInput{BaseInputTokens: 100, OutputTokens: 10})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Items) != 3 || res.Items[2].Var != "" || res.Items[2].Coef != 1000 {
		t.Fatalf("items = %+v", res.Items)
	}

	c, _ = Compile(`ctx <= 128000 ? p * 5 + c * 20 : p * 10 + c * 40`)
	res, _ = RunDetail(c, FormulaInput{BaseInputTokens: 100, OutputTokens: 10})
	if res.Items != nil {
		t.Fatalf("ternary without tier should not decompose: %+v", res.Items)
	}
}

// TestTierPerImageAndFacts 按张/按计量单位的线性形态
func TestTierPerImageAndFacts(t *testing.T) {
	c, _ := Compile(`tier("按张", fixed(0.04) * image_count)`)
	res, err := RunDetail(c, FormulaInput{ImageCount: 4})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].Var != "image_count" || res.Items[0].Quantity != 4 ||
		math.Abs(res.Items[0].Coef-40000) > 1e-9 {
		t.Fatalf("items = %+v", res.Items)
	}

	c, _ = Compile(`u("audio_seconds") * 400000`)
	res, _ = RunDetail(c, FormulaInput{Facts: map[string]float64{"audio_seconds": 60}})
	if len(res.Items) != 1 || res.Items[0].Var != "u:audio_seconds" || res.Items[0].Quantity != 60 {
		t.Fatalf("items = %+v", res.Items)
	}
}

// TestTierNonLinearFallback 非线性档位（min/max）只记录命中档位，不输出分项
func TestTierNonLinearFallback(t *testing.T) {
	c, err := Compile(`tier("封顶", min(p * 2 + c * 8, 1000000))`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	res, _ := RunDetail(c, FormulaInput{BaseInputTokens: 100, OutputTokens: 10})
	if len(res.Tiers) != 1 || res.Items != nil {
		t.Fatalf("tiers = %v items = %+v", res.Tiers, res.Items)
	}
}

// TestTimeFuncOptionalTZ 时间函数时区参数可省略（省略 = 平台计费时区）：此前签名强制 1 个参数，
// 文档声称可省略的 weekday()/hour() 写法求值即报 invalid number of arguments
func TestTimeFuncOptionalTZ(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	at := time.Date(2026, 11, 10, 10, 0, 0, 0, loc) // 周二 10:00（上海）
	base := `ctx > 300000 ? tier(">300K", p*1.5 + c*6) : tier("≤300K", p*1 + c*4)`
	rules := `weekday() >= 1 && weekday() <= 5 && hour() >= 9 && hour() < 18 ? 2 : 1`
	c, err := Compile("(" + base + ") * (" + rules + ")")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := SmokeTest(c.Src); err != nil {
		t.Fatalf("smoke: %v", err)
	}
	res, err := RunDetail(c, FormulaInput{BaseInputTokens: 10000, OutputTokens: 2000, Eval: &EvalContext{At: at, Location: loc}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// 工作时段规则被识别为请求规则（命中 ×2），档位分项保留原始单价
	if len(res.Rules) != 1 || !res.Rules[0].Matched || res.RuleMultiplier != 2 {
		t.Fatalf("rules = %+v ruleMul = %v", res.Rules, res.RuleMultiplier)
	}
	if len(res.Items) != 2 || res.Items[0].Coef != 1 || res.Items[1].Coef != 4 {
		t.Fatalf("items = %+v", res.Items)
	}
	// 显式时区与省略时区（平台时区）同值
	explicit, _ := Compile(`hour("Asia/Shanghai") == hour() ? 1 : 0`)
	if v, _, err := Run(explicit, FormulaInput{Eval: &EvalContext{At: at, Location: loc}}); err != nil || v != 1 {
		t.Fatalf("explicit vs default tz: v=%v err=%v", v, err)
	}
}

// TestUnreferencedCacheBilledInP 未引用的缓存 token 留在 p 按输入价计（此前被扣出 p 又不单独计价，等于免费）
func TestUnreferencedCacheBilledInP(t *testing.T) {
	in := FormulaInput{BaseInputTokens: 1000, OutputTokens: 100, CacheReadTokens: 400, CacheCreation5m: 200, CacheCreation1h: 50}

	c, _ := Compile(`tier("base", p * 2 + c * 8)`)
	res, err := RunDetail(c, in)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// p = 1000 + 400 + 200 + 50 = 1650
	if res.Factor != 1650*2+100*8 || res.Vars["p"] != float64(1650) {
		t.Fatalf("factor = %v vars = %v", res.Factor, res.Vars)
	}

	// 只引用 cr：cc/cc1h 仍留在 p
	c, _ = Compile(`tier("base", p * 2 + c * 8 + cr * 0.2)`)
	res, _ = RunDetail(c, in)
	if res.Factor != 1250*2+100*8+400*0.2 {
		t.Fatalf("cr only: factor = %v", res.Factor)
	}
}

// TestCC1hMergedIntoCC 引用 cc 未引用 cc1h：1h 写入按 cc 单价 ×1.6 计，账单拆成两行真实 token 数
func TestCC1hMergedIntoCC(t *testing.T) {
	in := FormulaInput{BaseInputTokens: 1000, OutputTokens: 100, CacheCreation5m: 200, CacheCreation1h: 50}
	c, _ := Compile(`tier("base", p * 2 + c * 8 + cc * 2.5)`)
	res, err := RunDetail(c, in)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := 1000*2.0 + 100*8 + 200*2.5 + 50*2.5*1.6
	if math.Abs(res.Factor-want) > 1e-9 {
		t.Fatalf("factor = %v, want %v", res.Factor, want)
	}
	if res.Vars["cc"] != float64(200) || res.Vars["cc1h"] != float64(50) {
		t.Fatalf("vars 应展示真实 token 数: %v", res.Vars)
	}
	if len(res.Items) != 4 || res.Items[2].Var != "cc" || res.Items[3].Var != "cc1h" ||
		res.Items[3].Quantity != 50 || math.Abs(res.Items[3].Coef-4) > 1e-9 {
		t.Fatalf("items = %+v", res.Items)
	}

	// 显式引用 cc1h：按其自身单价
	c, _ = Compile(`tier("base", p * 2 + c * 8 + cc * 2.5 + cc1h * 5)`)
	res, _ = RunDetail(c, in)
	if res.Factor != 1000*2+100*8+200*2.5+50*5 {
		t.Fatalf("explicit cc1h: factor = %v", res.Factor)
	}
}

// TestMultilineTierChainExamples AI 技能/编辑器模板采用的多行嵌套档位写法：
// 换行缩进可正常编译并通过保存烟测，命中档位与分项按 ctx 判定
func TestMultilineTierChainExamples(t *testing.T) {
	src := "ctx <= 32000\n  ? tier(\"short\", p * 1.2 + c * 6 + cr * 0.24 + cc * 1.5)\n  : (ctx <= 128000\n    ? tier(\"mid\", p * 2.4 + c * 12 + cr * 0.48 + cc * 3)\n    : tier(\"long\", p * 3 + c * 15 + cr * 0.6 + cc * 3.75))"
	if err := SmokeTest(src); err != nil {
		t.Fatalf("smoke: %v", err)
	}
	c, _ := Compile(src)
	// 缓存命中让 p 变小，但档位按 ctx（完整输入）判定
	res, err := RunDetail(c, FormulaInput{BaseInputTokens: 20000, CacheReadTokens: 50000, OutputTokens: 1000})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Tiers) != 1 || res.Tiers[0] != "mid" || res.Items == nil {
		t.Fatalf("tiers = %v items = %+v", res.Tiers, res.Items)
	}
}

// TestEvaluateWithContext 模拟请求套用到全部预置样例：保留样例名，规则按模拟请求命中
func TestEvaluateWithContext(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	ec := &EvalContext{
		At:       time.Date(2026, 11, 10, 10, 0, 0, 0, loc), // 周二 10:00
		Location: loc,
		Request:  &RequestInput{Params: map[string]any{"service_tier": "priority"}},
	}
	src := `(tier("base", p * 1 + c * 4)) * ((weekday() >= 1 && weekday() <= 5 && hour() >= 9 && hour() < 18 ? 2 : 1)
* (param("service_tier") == "priority" ? 1.5 : 1)
* ((param("duration") ?? 0) >= 10 ? 3 : 1))`
	_, results, err := EvaluateWithContext(src, ec)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(results) != len(SmokeVectors()) || results[0].Name != SmokeVectors()[0].Name {
		t.Fatalf("应保留全部预置样例名: %d %q", len(results), results[0].Name)
	}
	for _, r := range results {
		if len(r.Rules) != 3 || !r.Rules[0].Matched || !r.Rules[1].Matched || r.Rules[2].Matched {
			t.Fatalf("样例「%s」规则命中异常: %+v", r.Name, r.Rules)
		}
	}
	// 常规对话：(p=17000 含未引用缓存, c=2000) → (17000 + 8000) × 2 × 1.5 = 75000
	if results[1].Factor != 75000 {
		t.Fatalf("常规对话系数 = %v", results[1].Factor)
	}
}

// TestTierCompileErrors 档位名必须是非空字面量；_tier 为保留字
func TestTierCompileErrors(t *testing.T) {
	for _, src := range []string{
		`tier(p, p * 2)`,
		`tier("", p * 2)`,
		`tier("a")`,
		`_tier(0, p)`,
	} {
		if _, err := Compile(src); err == nil {
			t.Errorf("%s: expected compile error", src)
		}
	}
	if err := SmokeTest(`ctx <= 128000 ? tier("短", p * 2 + c * 8) : tier("长", p * 4 + c * 16)`); err != nil {
		t.Fatalf("smoke: %v", err)
	}
	if _, err := Compile(`tier("a", p) + tier("b", c)`); err != nil && !strings.Contains(err.Error(), "tier") {
		t.Fatalf("unexpected: %v", err)
	}
}
