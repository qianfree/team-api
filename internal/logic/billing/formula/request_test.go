package formula

import (
	"strings"
	"testing"
	"time"
)

// TestRequestProbeLiteralEnforcement param()/header() 字面量强制：
// 动态参数与空参数编译即拒绝（与 u() 同口径，任务路径冻结依赖静态可提取）
func TestRequestProbeLiteralEnforcement(t *testing.T) {
	cases := []struct {
		src  string
		fail bool
	}{
		{`header("x-api-version") == "v2" ? 2 : 1`, false},
		{`param("service_tier") == "priority" ? 2 : 1`, false},
		{`has(header("anthropic-beta"), "fast") ? 6 : 1`, false},
		{`hour("Asia/Shanghai") >= 23 ? 0.8 : 1`, false},
		// 常量拼接 key：编译期常量折叠为字面量，可静态提取，放行
		{`header("x-" + "v") == "v2" ? 2 : 1`, false},
		// 变量动态 key：无法静态提取探测集合，拒绝
		{`param(p) == 1 ? 2 : 1`, true},
		{`header(c) == "v2" ? 2 : 1`, true},
		// 空 key
		{`header("") == "v2" ? 2 : 1`, true},
		// 埋点保留字
		{`_trace(0, true, 2)`, true},
		{`_trace_int(0, true, 2)`, true},
	}
	for _, tc := range cases {
		_, err := Compile(tc.src)
		if tc.fail && err == nil {
			t.Fatalf("公式 %q 应当编译失败", tc.src)
		}
		if !tc.fail && err != nil {
			t.Fatalf("公式 %q 不应编译失败: %v", tc.src, err)
		}
	}
}

// TestExtractProbes 探测集合提取（去重、顺序、header/param 分桶）
func TestExtractProbes(t *testing.T) {
	c, err := Compile(`header("X-Ver") + header("x-ver") + param("service_tier") + param("metadata.image") + p`)
	if err != nil {
		t.Fatalf("编译失败: %v", err)
	}
	if len(c.HeaderKeys) != 2 || c.HeaderKeys[0] != "X-Ver" || c.HeaderKeys[1] != "x-ver" {
		t.Fatalf("HeaderKeys 提取错误: %v", c.HeaderKeys)
	}
	if len(c.ParamPaths) != 2 || c.ParamPaths[0] != "service_tier" || c.ParamPaths[1] != "metadata.image" {
		t.Fatalf("ParamPaths 提取错误: %v", c.ParamPaths)
	}
}

// TestRequestRuleTracePatching 埋点提取与命中回填：
//   - 形状严格「探针条件 ? 数字字面量 : 1」才提取（回退非 1 / 非探针条件不提取）
//   - 命中标记与数值结果正确；未引用（短路跳过）的规则保持未命中
func TestRequestRuleTracePatching(t *testing.T) {
	c, err := Compile(`(p * 5 + c * 10) * (header("tier") == "pro" ? 2 : 1) * (param("mode") == "fast" ? 3 : 1) * (p > 0 ? 5 : 1) * (header("x") == "y" ? 4 : 1.5)`)
	if err != nil {
		t.Fatalf("编译失败: %v", err)
	}
	// 只有前两个三目符合埋点形状：探针条件 + 回退 1
	if len(c.Rules) != 2 {
		t.Fatalf("应提取 2 条规则, got %d: %+v", len(c.Rules), c.Rules)
	}
	if c.Rules[0].Cond == "" || c.Rules[0].Multiplier != 2 || c.Rules[1].Multiplier != 3 {
		t.Fatalf("规则内容错误: %+v", c.Rules)
	}

	in := FormulaInput{
		BaseInputTokens: 100, OutputTokens: 50,
		Eval: &EvalContext{Request: &RequestInput{
			Headers: map[string]string{"tier": "pro"}, // 命中第 1 条;param("mode") 缺失 → 不命中第 2 条
		}},
	}
	factor, rules, err := Run(c, in)
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	// 数值 = (5*100 + 10*50) × 2 × 1 × 5 × 1.5（p>0 恒真取 5;header x≠y 取 1.5）
	want := float64(5*100+10*50) * 2 * 1 * 5 * 1.5
	if factor != want {
		t.Fatalf("系数 = %v, want %v", factor, want)
	}
	if !rules[0].Matched {
		t.Fatalf("第 1 条规则应命中: %+v", rules[0])
	}
	if rules[1].Matched {
		t.Fatalf("第 2 条规则不应命中: %+v", rules[1])
	}
}

// TestTraceShortCircuitSkippedRule 短路跳过的规则保持未命中(&& 左假时右侧条件不求值)
func TestTraceShortCircuitSkippedRule(t *testing.T) {
	c, err := Compile(`fixed(1) * (p > 0 && header("x") == "y" ? 2 : 1)`)
	if err != nil {
		t.Fatalf("编译失败: %v", err)
	}
	if len(c.Rules) != 1 {
		t.Fatalf("应提取 1 条规则, got %d", len(c.Rules))
	}
	// p = 0 → && 短路,header 条件不求值 → 规则未命中,数值取 1
	_, rules, err := Run(c, FormulaInput{})
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	if rules[0].Matched {
		t.Fatalf("短路跳过的规则不应标记命中: %+v", rules[0])
	}
}

// TestRequestProbeFuncs header/param/has 取值语义（缺失空值、大小写归一、nil 上下文）
func TestRequestProbeFuncs(t *testing.T) {
	run := func(src string, req *RequestInput) float64 {
		t.Helper()
		c, err := Compile(src)
		if err != nil {
			t.Fatalf("编译失败 %q: %v", src, err)
		}
		v, _, err := Run(c, FormulaInput{Eval: &EvalContext{Request: req}})
		if err != nil {
			t.Fatalf("求值失败 %q: %v", src, err)
		}
		return v
	}
	req := &RequestInput{
		Headers: map[string]string{"x-api-version": "v2", "anthropic-beta": "fast-mode-2026"},
		Params:  map[string]any{"service_tier": "priority", "n": float64(4)},
	}
	if v := run(`header("X-API-VERSION") == "v2" ? 1 : 0`, req); v != 1 {
		t.Fatalf("header 大小写归一失败: %v", v)
	}
	if v := run(`has(header("anthropic-beta"), "fast-mode") ? 1 : 0`, req); v != 1 {
		t.Fatalf("has 子串匹配失败: %v", v)
	}
	if v := run(`param("service_tier") == "priority" ? 1 : 0`, req); v != 1 {
		t.Fatalf("param 字符串比较失败: %v", v)
	}
	if v := run(`param("n") * 100`, req); v != 400 {
		t.Fatalf("param 数值参与算术失败: %v", v)
	}
	// 缺失探针：header 空、param nil → 条件 false
	if v := run(`header("missing") == "" ? 1 : 0`, req); v != 1 {
		t.Fatalf("缺失 header 应为空串: %v", v)
	}
	if v := run(`param("missing") == nil ? 1 : 0`, req); v != 1 {
		t.Fatalf("缺失 param 应为 nil: %v", v)
	}
	// nil 请求上下文（预扣前的兜底场景）
	if v := run(`header("x") == "" && param("y") == nil ? 1 : 0`, nil); v != 1 {
		t.Fatalf("nil 上下文探针应取空值: %v", v)
	}
}

// TestTimeFuncsBoundToEvalAt 时间函数绑定 EvalAt（非 now）与时区语义：
// 预扣与结算共用同一 At → 跨小时边界口径一致（new-api 取 time.Now() 会漂移）
func TestTimeFuncsBoundToEvalAt(t *testing.T) {
	// 2026-11-04 23:30 北京时间 = 15:30 UTC
	at := time.Date(2026, 11, 4, 15, 30, 0, 0, time.UTC)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	in := FormulaInput{Eval: &EvalContext{At: at, Location: loc}}

	run := func(src string) float64 {
		t.Helper()
		c, err := Compile(src)
		if err != nil {
			t.Fatalf("编译失败 %q: %v", src, err)
		}
		v, _, err := Run(c, in)
		if err != nil {
			t.Fatalf("求值失败 %q: %v", src, err)
		}
		return v
	}
	if v := run(`hour("")`); v != 23 { // 省略时区 → 默认平台时区（上海）
		t.Fatalf("hour 默认时区 = %v, want 23", v)
	}
	if v := run(`hour("UTC")`); v != 15 {
		t.Fatalf("hour UTC = %v, want 15", v)
	}
	if v := run(`weekday("Asia/Shanghai")`); v != 3 { // 2026-11-04 周三
		t.Fatalf("weekday = %v, want 3", v)
	}
	if v := run(`month("")`); v != 11 {
		t.Fatalf("month = %v, want 11", v)
	}
	if v := run(`day("UTC")`); v != 4 {
		t.Fatalf("day = %v, want 4", v)
	}
	// 夜间折扣跨零点组合：23 点命中
	if v := run(`(hour("") >= 23 || hour("") < 8) ? 0.8 : 1`); v != 0.8 {
		t.Fatalf("夜间折扣应命中: %v", v)
	}
	// 非法时区回退默认时区（不报错）
	if v := run(`hour("Mars/Olympus")`); v != 23 {
		t.Fatalf("非法时区应回退默认: %v", v)
	}
}

// TestTraceIntPreservedIntegerSemantics 整型乘数走 _trace_int，参与 % 等整数运算语义不变
// （若统一改写为 float 版 _trace，float64 % int 会直接报错）
func TestTraceIntPreservedIntegerSemantics(t *testing.T) {
	c, err := Compile(`(header("x") == "y" ? 3 : 1) % 2`)
	if err != nil {
		t.Fatalf("编译失败: %v", err)
	}
	v, rules, err := Run(c, FormulaInput{Eval: &EvalContext{Request: &RequestInput{Headers: map[string]string{"x": "y"}}}})
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	if len(rules) != 1 || !rules[0].Matched {
		t.Fatalf("整型规则应命中: %+v", rules)
	}
	if v != 1 { // 3 % 2
		t.Fatalf("整型语义错误: %v", v)
	}
	// 未命中分支同样保持整数运算（1 % 2 = 1）
	v, rules, err = Run(c, FormulaInput{})
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	if v != 1 || rules[0].Matched {
		t.Fatalf("未命中整型语义错误: %v %+v", v, rules)
	}
}

// TestRequestInputMapRoundTrip 任务路径冻结/解冻往返（ratios JSONB 一致性）
func TestRequestInputMapRoundTrip(t *testing.T) {
	src := `(header("tier") == "pro" ? 2 : 1) * (param("mode") == "fast" ? 3 : 1) * fixed(1)`
	c, err := Compile(src)
	if err != nil {
		t.Fatalf("编译失败: %v", err)
	}
	orig := &RequestInput{
		Headers: map[string]string{"tier": "pro"},
		Params:  map[string]any{"mode": "fast"},
	}
	thawed := RequestInputFromMap(orig.ToMap())
	f1, r1, err := Run(c, FormulaInput{Eval: &EvalContext{Request: orig}})
	if err != nil {
		t.Fatalf("求值失败: %v", err)
	}
	f2, r2, err := Run(c, FormulaInput{Eval: &EvalContext{Request: thawed}})
	if err != nil {
		t.Fatalf("重放求值失败: %v", err)
	}
	if f1 != f2 || f1 != 6_000_000 {
		t.Fatalf("冻结重放系数不一致: %v vs %v (want 6e6)", f1, f2)
	}
	if !r1[0].Matched || !r2[0].Matched || !r1[1].Matched || !r2[1].Matched {
		t.Fatalf("命中标记往返不一致: %+v vs %+v", r1, r2)
	}
	if RequestInputFromMap(nil) != nil {
		t.Fatalf("nil map 应还原为 nil")
	}
}

// TestSnapshotVarsIncludesProbes 快照变量包含 header:/param: 探测值
func TestSnapshotVarsIncludesProbes(t *testing.T) {
	c, err := Compile(`p * 5 + u("audio_seconds") * 4 + header("tier") == "pro" ? 2 : 1`)
	if err != nil {
		t.Fatalf("编译失败: %v", err)
	}
	vars := SnapshotVars(c, FormulaInput{
		BaseInputTokens: 10,
		Facts:           map[string]float64{"audio_seconds": 60},
		Eval:            &EvalContext{Request: &RequestInput{Headers: map[string]string{"tier": "pro"}}},
	})
	if vars["p"] != float64(10) || vars["u:audio_seconds"] != float64(60) {
		t.Fatalf("基础变量快照错误: %v", vars)
	}
	if vars["header:tier"] != "pro" {
		t.Fatalf("header 探测值缺失: %v", vars)
	}
	if _, ok := vars["fixed"]; ok {
		t.Fatalf("函数项不应进入快照: %v", vars)
	}
	// 编译缓存命中后 Src 一致（组合公式缓存键 = 原文 SHA-256）
	if c2, _ := Compile(`p * 5 + u("audio_seconds") * 4 + header("tier") == "pro" ? 2 : 1`); c2.Src != c.Src {
		t.Fatalf("缓存命中应返回同一编译产物")
	}
}

// TestCombinedFormulaSmoke 基础公式 × 请求规则的组合烟测（保存链路口径）
func TestCombinedFormulaSmoke(t *testing.T) {
	base := `p * 5 + c * 10`
	rules := `(header("tier") == "pro" ? 2 : 1) * (hour("Asia/Shanghai") >= 23 ? 0.8 : 1)`
	combined := "(" + base + ") * (" + rules + ")"
	if err := SmokeTest(combined); err != nil {
		t.Fatalf("组合公式烟测失败: %v", err)
	}
	// 空请求向量下（header 空、hour 取当前时刻任一分支）结果须仍非负有限
	c, err := Compile(combined)
	if err != nil {
		t.Fatalf("编译失败: %v", err)
	}
	if len(c.Rules) != 2 {
		t.Fatalf("组合公式应提取 2 条规则, got %d", len(c.Rules))
	}
	if !strings.Contains(c.Rules[0].Cond, `header("tier")`) {
		t.Fatalf("规则条件原文错误: %s", c.Rules[0].Cond)
	}
}
