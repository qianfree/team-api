package formula

import "testing"

// TestDescribeRuleCond 请求规则条件 → 中文描述：覆盖管理端可视化编辑器生成的全部形态与常见手写形态
func TestDescribeRuleCond(t *testing.T) {
	cases := []struct {
		rules string
		want  string
	}{
		{`(weekday() >= 1 && weekday() <= 5 && hour() >= 9 && hour() < 18 ? 2 : 1)`, "周一至周五 且 09:00–18:00"},
		{`(hour() >= 23 || hour() < 8 ? 0.8 : 1)`, "23:00–次日 08:00"},
		{`(weekday() == 0 || weekday() == 6 ? 0.9 : 1)`, "周六、周日"},
		{`(weekday() == 1 || weekday() == 3 || weekday() == 5 ? 2 : 1)`, "周一、周三、周五"},
		{`(param("service_tier") == "priority" ? 2 : 1)`, "参数 service_tier 等于 priority"},
		{`(has(header("anthropic-beta"), "fast-mode") ? 6 : 1)`, "请求头 anthropic-beta 包含 fast-mode"},
		{`((param("duration") ?? 0) >= 10 ? 1.5 : 1)`, "参数 duration 大于等于 10"},
		{`(param("duration") >= 10 ? 1.5 : 1)`, "参数 duration 大于等于 10"},
		{`(param("metadata.mode") != nil && header("x-vip") != "" ? 4 : 1)`, "参数 metadata.mode 存在 且 请求头 x-vip 存在"},
		{`(header("x-tier") != "free" && (param("max_tokens") ?? 0) < 1000 ? 0.7 : 1)`, "请求头 x-tier 不等于 free 且 参数 max_tokens 小于 1000"},
		{`(weekday("Asia/Tokyo") == 6 && (hour("Asia/Tokyo") * 60 + minute("Asia/Tokyo")) >= 510 && (hour("Asia/Tokyo") * 60 + minute("Asia/Tokyo")) < 735 ? 1.25 : 1)`, "周六 且 08:30–12:15（Asia/Tokyo）"},
		{`((hour() * 60 + minute()) >= 1350 || (hour() * 60 + minute()) < 405 ? 0.5 : 1)`, "22:30–次日 06:45"},
		{`(hour() >= 18 ? 3 : 1)`, "18:00–24:00"},
		{`(hour("Asia/Shanghai") >= 23 || hour("Asia/Shanghai") < 8 ? 0.8 : 1)`, "23:00–次日 08:00（Asia/Shanghai）"},
		{`(month() == 12 ? 0.8 : 1)`, "12 月"},
		{`(param("n") == 2 ? 1.1 : 1)`, "参数 n 等于 2"},
		// 无法描述：混用时区 / 非探针条件
		{`(hour("UTC") >= 9 && weekday("Asia/Shanghai") == 1 ? 2 : 1)`, ""},
		{`(len(param("messages")) > 10 ? 2 : 1)`, ""},
	}
	for _, tc := range cases {
		c, err := Compile("(p) * " + tc.rules)
		if err != nil {
			t.Fatalf("compile %s: %v", tc.rules, err)
		}
		if len(c.Rules) != 1 {
			t.Fatalf("%s: rules = %+v", tc.rules, c.Rules)
		}
		if got := c.Rules[0].Desc; got != tc.want {
			t.Errorf("%s\n got  %q\n want %q", tc.rules, got, tc.want)
		}
	}
}
