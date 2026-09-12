package shared

import (
	"encoding/json"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
)

// 服务端联网搜索（server-side web search）的跨协议中间表示。
//
// 四种文本协议都有「让模型自己去联网搜」的原生构件，但表达方式完全不同：
//
//	OpenAI chat  顶层参数   web_search_options: {search_context_size, user_location}
//	Claude       工具项     {type: "web_search_20250305", name: "web_search", max_uses, allowed_domains, ...}
//	Gemini       工具项     {googleSearch: {}}
//	Responses    工具项     {type: "web_search"}（旧版 web_search_preview）
//	Ollama       无          —— 真实能力缺失，只能降级
//
// 此前各方向要么原样丢弃、要么把它当成自定义函数伪造，导致**静默能力丢失**：
// 客户端明确要求联网、请求却以「没搜索」的方式成功返回，比直接报错更难排查。
// 本文件把它抽成一个格式无关的 spec：各转换器只负责「从源格式探测」与「产出目标格式」，
// 于是经 OpenAI 中枢的两跳链（如 Gemini→OpenAI→Claude）也能自动保住该能力
// —— chat 的 web_search_options 正是链路中枢的载体。

// claudeWebSearchToolType 产出 Claude web_search 工具时使用的版本化类型标识。
// Anthropic 的服务端工具按日期版本化，跨协议映射时取当前稳定版。
const claudeWebSearchToolType = "web_search_20250305"

// claudeWebSearchToolName Claude web_search 工具的固定名称。
const claudeWebSearchToolName = "web_search"

// WebSearchSpec 服务端联网搜索的格式无关描述。
// 除 Enabled 外的字段是各协议的可选约束，目标格式无对应物时静默降级
// （能力本身保住，细粒度约束丢失——这与「整个搜索能力丢失」是两个量级）。
type WebSearchSpec struct {
	// MaxUses 最大搜索次数（Claude 独有）
	MaxUses *int
	// AllowedDomains / BlockedDomains 域名白/黑名单（Claude 独有）
	AllowedDomains []string
	BlockedDomains []string
	// UserLocation 地理位置提示（Claude 与 OpenAI chat 均支持，形态不同，原样透传）
	UserLocation any
	// SearchContextSize 搜索上下文规模 low/medium/high（OpenAI chat 独有）
	SearchContextSize string
}

// DetectWebSearchFromOpenAI 从 chat 请求的 web_search_options 探测搜索能力。
// 该字段存在（哪怕是空对象 {}）即表示启用——这是 OpenAI 的约定。
func DetectWebSearchFromOpenAI(opts json.RawMessage) *WebSearchSpec {
	if len(opts) == 0 || string(opts) == "null" {
		return nil
	}
	spec := &WebSearchSpec{}
	var parsed struct {
		SearchContextSize string `json:"search_context_size"`
		UserLocation      any    `json:"user_location"`
	}
	if err := json.Unmarshal(opts, &parsed); err == nil {
		spec.SearchContextSize = parsed.SearchContextSize
		spec.UserLocation = parsed.UserLocation
	}
	return spec
}

// DetectWebSearchFromClaudeTools 从 Claude tools 中探测 web_search 内置工具。
// 内置工具带日期版本后缀（web_search_20250305 等），按前缀识别。
func DetectWebSearchFromClaudeTools(tools []dto.ClaudeTool) *WebSearchSpec {
	for _, t := range tools {
		if !strings.HasPrefix(t.Type, "web_search") {
			continue
		}
		return &WebSearchSpec{
			MaxUses:        t.MaxUses,
			AllowedDomains: t.AllowedDomains,
			BlockedDomains: t.BlockedDomains,
			UserLocation:   t.UserLocation,
		}
	}
	return nil
}

// DetectWebSearchFromGeminiTools 从 Gemini tools 中探测 googleSearch（含旧版
// googleSearchRetrieval）条目。tools 为 repeated Tool，需逐条目检查。
func DetectWebSearchFromGeminiTools(toolsJSON json.RawMessage) *WebSearchSpec {
	if len(toolsJSON) == 0 {
		return nil
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(toolsJSON, &entries); err != nil {
		return nil
	}
	for _, entry := range entries {
		if _, ok := entry["googleSearch"]; ok {
			return &WebSearchSpec{}
		}
		if _, ok := entry["googleSearchRetrieval"]; ok {
			return &WebSearchSpec{}
		}
	}
	return nil
}

// DetectWebSearchFromResponsesTools 从 Responses tools 中探测 web_search 工具// （含 web_search_preview / web_search_2025xx 等变体，按前缀识别）。
func DetectWebSearchFromResponsesTools(toolsJSON json.RawMessage) *WebSearchSpec {
	if len(toolsJSON) == 0 {
		return nil
	}
	var tools []map[string]any
	if err := json.Unmarshal(toolsJSON, &tools); err != nil {
		return nil
	}
	for _, tool := range tools {
		typ, _ := tool["type"].(string)
		if !strings.HasPrefix(typ, "web_search") {
			continue
		}
		spec := &WebSearchSpec{}
		if loc, ok := tool["user_location"]; ok {
			spec.UserLocation = loc
		}
		if size, ok := tool["search_context_size"].(string); ok {
			spec.SearchContextSize = size
		}
		return spec
	}
	return nil
}

// ToOpenAIOptions 产出 chat 的 web_search_options。
// 无可表达的约束时产出空对象 {}——对 OpenAI 而言「字段存在」即启用。
func (s *WebSearchSpec) ToOpenAIOptions() json.RawMessage {
	if s == nil {
		return nil
	}
	opts := map[string]any{}
	if s.SearchContextSize != "" {
		opts["search_context_size"] = s.SearchContextSize
	}
	if s.UserLocation != nil {
		opts["user_location"] = s.UserLocation
	}
	raw, err := json.Marshal(opts)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

// ToClaudeTool 产出 Claude 的 web_search 内置工具项。
func (s *WebSearchSpec) ToClaudeTool() dto.ClaudeTool {
	return dto.ClaudeTool{
		Type:           claudeWebSearchToolType,
		Name:           claudeWebSearchToolName,
		MaxUses:        s.MaxUses,
		AllowedDomains: s.AllowedDomains,
		BlockedDomains: s.BlockedDomains,
		UserLocation:   s.UserLocation,
	}
}

// ToResponsesTool 产出 Responses 的 web_search 工具项。
func (s *WebSearchSpec) ToResponsesTool() map[string]any {
	tool := map[string]any{"type": "web_search"}
	if s.UserLocation != nil {
		tool["user_location"] = s.UserLocation
	}
	if s.SearchContextSize != "" {
		tool["search_context_size"] = s.SearchContextSize
	}
	return tool
}

// GeminiGoogleSearchEntry 产出 Gemini tools 数组中的 googleSearch 条目。
// Gemini 侧无任何可承载的约束字段，MaxUses / 域名白名单等一律静默降级。
func GeminiGoogleSearchEntry() map[string]any {
	return map[string]any{"googleSearch": map[string]any{}}
}
