package helper

import "testing"

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		name         string
		channelType  int
		baseURL      string
		settingsJSON string
		want         string
	}{
		// 自动（默认）：命中已知版本段 → 剥离成站点根
		{name: "auto_尾部斜杠先剥离", channelType: 1, baseURL: "https://api.openai.com/", settingsJSON: `{}`, want: "https://api.openai.com"},
		{name: "auto_openai带v1", channelType: 1, baseURL: "https://my-proxy.com/v1", settingsJSON: `{}`, want: "https://my-proxy.com"},
		{name: "auto_gemini带v1beta", channelType: 3, baseURL: "https://generativelanguage.googleapis.com/v1beta", settingsJSON: `{}`, want: "https://generativelanguage.googleapis.com"},
		{name: "auto_智谱官方完整前缀", channelType: 7, baseURL: "https://open.bigmodel.cn/api/paas/v4", settingsJSON: `{}`, want: "https://open.bigmodel.cn"},
		{name: "auto_智谱anthropic前缀", channelType: 7, baseURL: "https://open.bigmodel.cn/api/anthropic", settingsJSON: `{}`, want: "https://open.bigmodel.cn"},
		{name: "auto_火山官方apiV3", channelType: 10, baseURL: "https://ark.cn-beijing.volces.com/api/v3", settingsJSON: `{}`, want: "https://ark.cn-beijing.volces.com"},
		{name: "auto_火山旧默认值带api尾", channelType: 10, baseURL: "https://ark.cn-beijing.volces.com/api", settingsJSON: `{}`, want: "https://ark.cn-beijing.volces.com"},
		{name: "auto_阿里兼容模式", channelType: 4, baseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", settingsJSON: `{}`, want: "https://dashscope.aliyuncs.com"},
		{name: "auto_百度千帆v2", channelType: 19, baseURL: "https://qianfan.baidubce.com/v2", settingsJSON: `{}`, want: "https://qianfan.baidubce.com"},
		{name: "auto_openrouter多段前缀优先于v1", channelType: 27, baseURL: "https://openrouter.ai/api/v1", settingsJSON: `{}`, want: "https://openrouter.ai"},
		{name: "auto_deepseek的anthropic_v1整体剥离", channelType: 8, baseURL: "https://api.deepseek.com/anthropic/v1", settingsJSON: `{}`, want: "https://api.deepseek.com"},

		// 自动：不命中任何已知后缀 → 原样保留（含深路径中转站根）
		{name: "auto_深路径代理根不误剥", channelType: 1, baseURL: "https://gateway.internal/teams/openai", settingsJSON: `{}`, want: "https://gateway.internal/teams/openai"},
		{name: "auto_非版本结尾不动", channelType: 2, baseURL: "https://api.anthropic.com", settingsJSON: `{}`, want: "https://api.anthropic.com"},

		// root：显式站点根，绝不剥离（代理站点根本身以 /v1 结尾的极端场景逃生口）
		{name: "root_强制原样", channelType: 1, baseURL: "https://weird-proxy.com/v1", settingsJSON: `{"base_url_type":"root"}`, want: "https://weird-proxy.com/v1"},

		// versioned：与 auto 同表剥离，声明语义留痕
		{name: "versioned_显式剥离", channelType: 3, baseURL: "https://gemini-proxy.com/v1beta", settingsJSON: `{"base_url_type":"versioned"}`, want: "https://gemini-proxy.com"},

		// JS 任务插件渠道：路径由插件自行约定，一律不归一化
		{name: "js插件渠道不剥离", channelType: 43, baseURL: "https://plugin-host.com/v1", settingsJSON: `{"base_url_type":"versioned"}`, want: "https://plugin-host.com/v1"},

		// 边界：空地址 / 非法 settings
		{name: "空地址返回空", channelType: 1, baseURL: "", settingsJSON: `{}`, want: ""},
		{name: "settings非法JSON按自动", channelType: 1, baseURL: "https://x.com/v1", settingsJSON: "not-json", want: "https://x.com"},

		// 未登记类型回落 /v1
		{name: "未登记类型回落v1", channelType: 26, baseURL: "https://spark-api-open.xf-yun.com/v1", settingsJSON: `{}`, want: "https://spark-api-open.xf-yun.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeBaseURL(c.channelType, c.baseURL, c.settingsJSON); got != c.want {
				t.Fatalf("NormalizeBaseURL(%d, %q, %q) = %q, want %q", c.channelType, c.baseURL, c.settingsJSON, got, c.want)
			}
		})
	}
}
