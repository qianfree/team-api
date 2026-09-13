package billing

import (
	"encoding/json"
	"testing"
)

// ── lookupPath 路径提取 ──

func TestLookupPath(t *testing.T) {
	var root any
	json.Unmarshal([]byte(`{
		"model": "m",
		"seconds": "8",
		"metadata": {
			"image": "https://x",
			"duration": 8,
			"resolution": null,
			"content": [
				{"type": "text", "text": "hi"},
				{"type": "image_url", "image_url": {"url": "a"}, "role": "first_frame"},
				{"type": "video_url", "video_url": {"url": "v"}}
			]
		}
	}`), &root)

	cases := []struct {
		path  string
		found bool
		count int
	}{
		{"model", true, 1},
		{"seconds", true, 1},
		{"metadata.image", true, 1},
		{"metadata.duration", true, 1},
		{"metadata.resolution", true, 1}, // 存在但值为 null
		{"metadata.missing", false, 0},
		{"metadata", true, 1},
		{"metadata.content", true, 1},
		{"metadata.content[*].type", true, 3},
		{"metadata.content[*].video_url.url", true, 1},
		{"metadata.content[*].role", true, 1},
		{"metadata.content[*].missing", false, 0},
		{"", false, 0},
		{"metadata..image", false, 0},
	}
	for _, c := range cases {
		values, found := lookupPath(root, c.path)
		if found != c.found {
			t.Errorf("lookupPath(%q) found=%v want %v", c.path, found, c.found)
			continue
		}
		if found && len(values) != c.count {
			t.Errorf("lookupPath(%q) values=%d want %d", c.path, len(values), c.count)
		}
	}
}

// ── 匹配语义 ──

func evalOne(t *testing.T, body string, conds ...ParamCondition) bool {
	t.Helper()
	return ruleConditionsMatch(conds, jsonMustParse(t, body))
}

func jsonMustParse(t *testing.T, s string) any {
	t.Helper()
	var root any
	if err := json.Unmarshal([]byte(s), &root); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return root
}

func TestMatchHasValue(t *testing.T) {
	// false/0/空串均为"有值"（与未传语义不同）；null 不算
	if !evalOne(t, `{"watermark": false}`, ParamCondition{Path: "watermark", Match: "has_value"}) {
		t.Error("false should count as has_value")
	}
	if !evalOne(t, `{"x": ""}`, ParamCondition{Path: "x", Match: "has_value"}) {
		t.Error("empty string should count as has_value")
	}
	if !evalOne(t, `{"x": 0}`, ParamCondition{Path: "x", Match: "has_value"}) {
		t.Error("zero should count as has_value")
	}
	if evalOne(t, `{"x": null}`, ParamCondition{Path: "x", Match: "has_value"}) {
		t.Error("null should NOT count as has_value")
	}
	if evalOne(t, `{}`, ParamCondition{Path: "x", Match: "has_value"}) {
		t.Error("missing key should not match")
	}
	// 数组：任一元素非 null
	if !evalOne(t, `{"content":[{"video_url":null},{"video_url":{"url":"v"}}]}`,
		ParamCondition{Path: "content[*].video_url", Match: "has_value"}) {
		t.Error("any non-null element should match")
	}
}

func TestMatchEquals(t *testing.T) {
	// 数字强转："8" == 8
	if !evalOne(t, `{"seconds": "8"}`, ParamCondition{Path: "seconds", Match: "equals", Value: float64(8)}) {
		t.Error(`"8" should equal 8`)
	}
	if !evalOne(t, `{"mode": "pro"}`, ParamCondition{Path: "mode", Match: "equals", Value: "pro"}) {
		t.Error("string equals")
	}
	if !evalOne(t, `{"flag": true}`, ParamCondition{Path: "flag", Match: "equals", Value: true}) {
		t.Error("bool equals")
	}
	if evalOne(t, `{"mode": "std"}`, ParamCondition{Path: "mode", Match: "equals", Value: "pro"}) {
		t.Error("different string should not equal")
	}
}

func TestMatchComparisons(t *testing.T) {
	cond := func(match string, val float64) ParamCondition {
		return ParamCondition{Path: "metadata.duration", Match: match, Value: val}
	}
	body := `{"metadata":{"duration":8}}`
	for _, m := range []string{"gt", "gte", "lt", "lte"} {
		var want bool
		switch m {
		case "gt":
			want = false
		case "gte":
			want = true
		case "lt":
			want = false
		case "lte":
			want = true
		}
		if got := evalOne(t, body, cond(m, 8)); got != want {
			t.Errorf("%s 8 vs 8 = %v want %v", m, got, want)
		}
	}
	// 字符串数字强转参与比较
	if !evalOne(t, `{"seconds":"12"}`, ParamCondition{Path: "seconds", Match: "gte", Value: float64(10)}) {
		t.Error(`"12" gte 10 should match`)
	}
	// 非数字字符串不参与比较
	if evalOne(t, `{"seconds":"abc"}`, ParamCondition{Path: "seconds", Match: "gte", Value: float64(10)}) {
		t.Error("non-numeric string should not match comparison")
	}
}

func TestMatchBetween(t *testing.T) {
	cond := ParamCondition{Path: "metadata.duration", Match: "between", Value: []any{float64(8), float64(12)}}
	if !evalOne(t, `{"metadata":{"duration":8}}`, cond) {
		t.Error("between should be inclusive of min")
	}
	if !evalOne(t, `{"metadata":{"duration":12}}`, cond) {
		t.Error("between should be inclusive of max")
	}
	if evalOne(t, `{"metadata":{"duration":7}}`, cond) {
		t.Error("below min should not match")
	}
	// 数组任一元素落入区间
	if !evalOne(t, `{"arr":[{"d":3},{"d":9}]}`, ParamCondition{Path: "arr[*].d", Match: "between", Value: []any{float64(8), float64(12)}}) {
		t.Error("any element in range should match")
	}
}

// ── 规则求值 ──

func TestEvalParamMultipliers(t *testing.T) {
	rules := []ParamRule{
		{
			// 图生视频：metadata.image 有值 → 1.5
			Conditions: []ParamCondition{{Path: "metadata.image", Match: "has_value"}},
			Multiplier: 1.5,
			Note:       "图生视频",
		},
		{
			// 隐式 AND：1080p 且 ≥10 秒 → 2.0
			Conditions: []ParamCondition{
				{Path: "metadata.resolution", Match: "equals", Value: "1080p"},
				{Path: "metadata.duration", Match: "gte", Value: float64(10)},
			},
			Multiplier: 2.0,
			Note:       "1080p 长视频",
		},
		{
			// 视频输入折扣
			Conditions: []ParamCondition{{Path: "metadata.content[*].video_url", Match: "has_value"}},
			Multiplier: 0.609,
			Note:       "视频输入折扣",
		},
	}

	// 命中规则 1（×1.5），规则 2 分辨率不满足
	m, matched := EvalParamMultipliers(rules, []byte(`{"model":"m","metadata":{"image":"https://x","resolution":"720p","duration":5}}`))
	if m != 1.5 || len(matched) != 1 || matched[0] != "图生视频" {
		t.Fatalf("case1: m=%v matched=%v", m, matched)
	}

	// 命中规则 1 + 规则 2（连乘 3.0）
	m, matched = EvalParamMultipliers(rules, []byte(`{"metadata":{"image":"i","resolution":"1080p","duration":12}}`))
	if m != 3.0 || len(matched) != 2 {
		t.Fatalf("case2: m=%v matched=%v", m, matched)
	}

	// 全不命中 → 1.0 / nil
	m, matched = EvalParamMultipliers(rules, []byte(`{"model":"m"}`))
	if m != 1.0 || matched != nil {
		t.Fatalf("case3: m=%v matched=%v", m, matched)
	}

	// AND 语义：一个条件不满足整条不命中
	m, _ = EvalParamMultipliers(rules, []byte(`{"metadata":{"image":"i","resolution":"1080p","duration":5}}`))
	if m != 1.5 {
		t.Fatalf("case4: partial-AND should only match rule1, m=%v", m)
	}

	// 视频输入折扣（[*] 路径）
	m, matched = EvalParamMultipliers(rules, []byte(`{"metadata":{"content":[{"type":"video_url","video_url":{"url":"v"}}]}}`))
	if m != 0.609 || len(matched) != 1 {
		t.Fatalf("case5: m=%v matched=%v", m, matched)
	}

	// 无规则 / 空 body / 非法 JSON → 1.0
	if m, _ := EvalParamMultipliers(nil, []byte(`{}`)); m != 1.0 {
		t.Error("no rules should be 1.0")
	}
	if m, _ := EvalParamMultipliers(rules, nil); m != 1.0 {
		t.Error("nil body should be 1.0")
	}
	if m, _ := EvalParamMultipliers(rules, []byte(`not-json`)); m != 1.0 {
		t.Error("invalid json should be 1.0")
	}
}

func TestEvalParamMultipliers_MultiplierProductOrder(t *testing.T) {
	// 无 note 时命中说明回退为路径拼接
	rules := []ParamRule{
		{Conditions: []ParamCondition{{Path: "a", Match: "has_value"}}, Multiplier: 2.0},
		{Conditions: []ParamCondition{{Path: "b", Match: "has_value"}}, Multiplier: 3.0},
	}
	m, matched := EvalParamMultipliers(rules, []byte(`{"a":1,"b":1}`))
	if m != 6.0 || len(matched) != 2 || matched[0] != "a" {
		t.Fatalf("m=%v matched=%v", m, matched)
	}
}

// ── 校验 ──

func TestValidateParamMultipliers(t *testing.T) {
	ok := []ParamRule{{Conditions: []ParamCondition{{Path: "a", Match: "gte", Value: float64(1)}}, Multiplier: 1.5}}
	if err := ValidateParamMultipliers(ok); err != nil {
		t.Fatalf("valid rules rejected: %v", err)
	}

	bad := []struct {
		name string
		rule ParamRule
	}{
		{"空条件", ParamRule{Conditions: nil, Multiplier: 1.5}},
		{"倍率为零", ParamRule{Conditions: []ParamCondition{{Path: "a", Match: "has_value"}}, Multiplier: 0}},
		{"倍率超上限", ParamRule{Conditions: []ParamCondition{{Path: "a", Match: "has_value"}}, Multiplier: 101}},
		{"路径为空", ParamRule{Conditions: []ParamCondition{{Path: "", Match: "has_value"}}, Multiplier: 1.5}},
		{"匹配方式非法", ParamRule{Conditions: []ParamCondition{{Path: "a", Match: "regex"}}, Multiplier: 1.5}},
		{"gte 缺值", ParamRule{Conditions: []ParamCondition{{Path: "a", Match: "gte"}}, Multiplier: 1.5}},
		{"gte 非数字", ParamRule{Conditions: []ParamCondition{{Path: "a", Match: "gte", Value: "abc"}}, Multiplier: 1.5}},
		{"between 非数组", ParamRule{Conditions: []ParamCondition{{Path: "a", Match: "between", Value: float64(1)}}, Multiplier: 1.5}},
		{"between 单元素", ParamRule{Conditions: []ParamCondition{{Path: "a", Match: "between", Value: []any{float64(1)}}}, Multiplier: 1.5}},
	}
	for _, c := range bad {
		if err := ValidateParamMultipliers([]ParamRule{c.rule}); err == nil {
			t.Errorf("%s: should be rejected", c.name)
		}
	}
}

// ── EstimateTaskCost 注入集成 ──

func TestEstimateTaskCost_ParamMultiplierInjection(t *testing.T) {
	// per_second 模型 + 参数倍率规则：命中注入 ratios 且费用连乘
	pricing := &PricingResult{
		BillingMode:      "per_second",
		TenantMultiplier: 1.0,
		PerSecondPrices:  map[string]float64{"720p": 0.5, "*": 0.5},
		ParamMultipliers: []ParamRule{
			{Conditions: []ParamCondition{{Path: "metadata.image", Match: "has_value"}}, Multiplier: 1.5, Note: "图生视频"},
			{Conditions: []ParamCondition{{Path: "seconds", Match: "gte", Value: float64(10)}}, Multiplier: 2.0, Note: "长视频"},
		},
	}
	// 直接调纯函数路径（estimateTaskCost 只吃 ratios）——先验证求值注入逻辑的等价小函数
	ratios := map[string]any{"spec.duration": 12.0, "spec.resolution": "720p"}
	body := []byte(`{"model":"m","prompt":"p","seconds":"12","metadata":{"image":"https://x","duration":12,"resolution":"720p"}}`)

	if m, matched := EvalParamMultipliers(pricing.ParamMultipliers, body); m != 3.0 || len(matched) != 2 {
		t.Fatalf("eval: m=%v matched=%v", m, matched)
	}
	// 模拟 EstimateTaskCost 内部注入（保持与实现一致的键名）
	if m, matched := EvalParamMultipliers(pricing.ParamMultipliers, body); len(matched) > 0 && m > 0 {
		ratios[ratioKeyParamMultiplier] = m
		ratios[ratioKeyParamMatched] = matched[0]
	}
	// 注入后费用 = 0.5 × 12 × 1.5 × 2.0 = 18
	assertDecimal(t, estimateTaskCost(pricing, ratios, nil), 18.0, "param multiplier cost")

	// 未命中的任务体不注入（保持 0.5 × 5 缺省 = 2.5）
	ratios2 := map[string]any{}
	assertDecimal(t, estimateTaskCost(pricing, ratios2, nil), 2.5, "no param match cost")
}
