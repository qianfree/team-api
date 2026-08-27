package kitutil

import "strings"

// GeminiUsesThinkingLevel 判断模型走 thinkingLevel（Gemini 3 及以后）还是
// thinkingBudget（Gemini 2.5 系）。
//
// 两个字段在 Gemini API 中互斥：同时下发上游返回 400
// "thinking_budget and thinking_level are not supported together"。
// 判据取模型名主版本号（gemini-<major>[.<minor>]-...），>= 3 走 level；
// 无法识别的模型名保守回退 budget（2.5 系语义，历史行为）。
func GeminiUsesThinkingLevel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	// 去掉 models/xxx、publishers/google/models/xxx 之类的路径前缀
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	if !strings.HasPrefix(m, "gemini-") {
		return false
	}
	rest := strings.TrimPrefix(m, "gemini-")
	major := 0
	digits := 0
	for _, r := range rest {
		if r < '0' || r > '9' {
			break
		}
		major = major*10 + int(r-'0')
		digits++
	}
	if digits == 0 {
		return false
	}
	return major >= 3
}

// GeminiThinkingLevelOf 将 effort 等级映射为 Gemini thinkingLevel 枚举值。
// Gemini 仅接受 LOW/MEDIUM/HIGH 三档，网关侧的 minimal/xhigh/max/ultra 折叠到最近档位。
// 返回空串表示不设置（none —— 由调用方决定是否整体跳过 thinking 配置）。
func GeminiThinkingLevelOf(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "none":
		return ""
	case "minimal", "low":
		return "LOW"
	case "medium":
		return "MEDIUM"
	case "high", "xhigh", "max", "ultra":
		return "HIGH"
	case "":
		// 无 effort 信息（如仅 -thinking 后缀）：取中档，与 budget 路径的默认强度对齐
		return "MEDIUM"
	default:
		return "MEDIUM"
	}
}
