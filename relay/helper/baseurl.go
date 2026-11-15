package helper

import (
	"encoding/json"
	"strings"

	"github.com/qianfree/team-api/relay/constant"
)

// providerStrippableSuffixes 各供应商 base_url 可剥离的版本段后缀（最长优先匹配）。
// 后缀集合 = 各 adaptor GetRequestURL 自行拼接的路径前缀（如 openai 拼 /v1/chat/completions、
// gemini 拼 /v1beta/models/...、智谱拼 /api/paas/v4/chat/completions）：
// 用户把官方文档给的 base_url（通常已带版本段）粘进渠道配置时，剥离后还原成
// adaptor 期望的「站点根」形态。未登记的类型回落到 ["/v1"]（OpenAI 兼容系的通用约定）。
var providerStrippableSuffixes = map[constant.ProviderType][]string{
	constant.ProviderGemini:     {"/v1beta", "/v1"},
	constant.ProviderAli:        {"/compatible-mode/v1", "/apps/anthropic/v1", "/apps/anthropic", "/api/v1"},
	constant.ProviderZhipu:      {"/api/paas/v4", "/api/anthropic/v1", "/api/anthropic"},
	constant.ProviderDeepSeek:   {"/anthropic/v1", "/anthropic", "/v1"},
	constant.ProviderMoonshot:   {"/anthropic/v1", "/anthropic", "/v1"},
	constant.ProviderMiniMax:    {"/anthropic/v1", "/anthropic", "/v1"},
	constant.ProviderVolcengine: {"/api/coding/v1", "/api/v3", "/api"},
	constant.ProviderOpenRouter: {"/api/v1", "/v1"},
	constant.ProviderBaiduV2:    {"/v2"},
	constant.ProviderJimeng:     {"/v2"},
}

// NormalizeBaseURL 将渠道配置的 base_url 归一化为「站点根」形态（各 adaptor 拼 URL 的统一契约）。
// settingsJSON 为 chn_channels.settings JSONB 原文，填写形态取其中的 base_url_type：
//   - ""（默认）=自动：以该供应商已知版本段结尾时剥离；
//   - "root"=站点根（域名），原样使用；
//   - "versioned"=已含版本前缀，按表剥离。
//
// auto 与 versioned 走同一张后缀表：版本段集合是封闭的（adaptor 拼什么路径是确定的），
// 与其按「最后一段长得像版本号」猜测，不如命中已知后缀才剥离，误剥率最低。
// JS 任务插件渠道（type=43）不参与归一化：上游路径由插件自行约定。
// 未命中任何后缀时原样返回——归一化是尽力而为的纠偏，不是强校验，异常交给上游报错暴露。
func NormalizeBaseURL(channelType int, baseURL string, settingsJSON string) string {
	baseURL = strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ""
	}
	if constant.ProviderType(channelType) == constant.ProviderJSTask {
		return baseURL
	}
	if parseBaseURLType(settingsJSON) == "root" {
		return baseURL
	}
	t := constant.ProviderType(channelType)
	for _, suffix := range strippableSuffixes(t) {
		if strings.HasSuffix(baseURL, suffix) {
			return strings.TrimSuffix(baseURL, suffix)
		}
	}
	return baseURL
}

// strippableSuffixes 返回供应商的可剥离后缀表；未登记的类型回落到 OpenAI 兼容系通用约定 /v1
func strippableSuffixes(t constant.ProviderType) []string {
	if v, ok := providerStrippableSuffixes[t]; ok {
		return v
	}
	return []string{"/v1"}
}

// parseBaseURLType 从 settings JSONB 原文中读取 base_url_type；解析失败按自动处理
func parseBaseURLType(settingsJSON string) string {
	if settingsJSON == "" {
		return ""
	}
	var s struct {
		BaseURLType string `json:"base_url_type"`
	}
	if err := json.Unmarshal([]byte(settingsJSON), &s); err != nil {
		return ""
	}
	return s.BaseURLType
}
