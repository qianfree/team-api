package constant

// ProviderType 定义 AI 供应商类型，与 chn_channels.type 字段对应
type ProviderType int

const (
	ProviderOpenAI      ProviderType = 1
	ProviderClaude      ProviderType = 2
	ProviderGemini      ProviderType = 3
	ProviderAli         ProviderType = 4
	ProviderTencent     ProviderType = 6
	ProviderZhipu       ProviderType = 7
	ProviderDeepSeek    ProviderType = 8
	ProviderMoonshot    ProviderType = 9
	ProviderVolcengine  ProviderType = 10
	ProviderAWS         ProviderType = 11
	ProviderAzure       ProviderType = 12
	ProviderVertex      ProviderType = 13
	ProviderMistral     ProviderType = 15
	ProviderXAI         ProviderType = 16
	ProviderAI360       ProviderType = 17
	ProviderLingyi      ProviderType = 18
	ProviderBaiduV2     ProviderType = 19
	ProviderCloudflare  ProviderType = 20
	ProviderOllama      ProviderType = 22
	ProviderSiliconFlow ProviderType = 25
	ProviderXunfei      ProviderType = 26
	ProviderOpenRouter  ProviderType = 27
	ProviderXInference  ProviderType = 28
	ProviderMiniMax     ProviderType = 29
	ProviderSubmodel    ProviderType = 30
	// 32 = Coze（扣子）、33 = Dify：渠道已于 v0.2 移除，编号永久保留不再复用，
	// 避免历史 bil_usage_logs / aud_request_logs 中的 channel_type 被新渠道串味。
	ProviderJimeng     ProviderType = 34
	ProviderCodex      ProviderType = 35
	ProviderSora       ProviderType = 37
	ProviderKling      ProviderType = 38
	ProviderSuno       ProviderType = 39
	ProviderMidjourney ProviderType = 40
	ProviderNewAPI     ProviderType = 41
	ProviderSub2API    ProviderType = 42
)

// String 返回供应商类型名称
func (p ProviderType) String() string {
	switch p {
	case ProviderOpenAI:
		return "OpenAI"
	case ProviderClaude:
		return "Claude"
	case ProviderGemini:
		return "Gemini"
	case ProviderAli:
		return "Ali"
	case ProviderTencent:
		return "Tencent"
	case ProviderZhipu:
		return "Zhipu"
	case ProviderDeepSeek:
		return "DeepSeek"
	case ProviderMoonshot:
		return "Moonshot"
	case ProviderVolcengine:
		return "Volcengine"
	case ProviderAWS:
		return "AWS Bedrock"
	case ProviderAzure:
		return "Azure OpenAI"
	case ProviderVertex:
		return "Vertex AI"
	case ProviderMistral:
		return "Mistral"
	case ProviderXAI:
		return "xAI"
	case ProviderAI360:
		return "360"
	case ProviderLingyi:
		return "Lingyi"
	case ProviderBaiduV2:
		return "Baidu V2"
	case ProviderCloudflare:
		return "Cloudflare"
	case ProviderOllama:
		return "Ollama"
	case ProviderSiliconFlow:
		return "SiliconFlow"
	case ProviderXunfei:
		return "Xunfei"
	case ProviderOpenRouter:
		return "OpenRouter"
	case ProviderXInference:
		return "XInference"
	case ProviderMiniMax:
		return "MiniMax"
	case ProviderSubmodel:
		return "Submodel"
	case ProviderJimeng:
		return "Jimeng"
	case ProviderCodex:
		return "Codex"
	case ProviderSora:
		return "Sora"
	case ProviderKling:
		return "Kling"
	case ProviderSuno:
		return "Suno"
	case ProviderMidjourney:
		return "Midjourney"
	case ProviderNewAPI:
		return "New API"
	case ProviderSub2API:
		return "Sub2API"
	default:
		return "Unknown"
	}
}

// IsMultiNativeProvider 判断是否为多协议原生透传供应商。
// 这类供应商（New API / Sub2API）的上游同时原生支持 OpenAI / Claude / Gemini 协议，
// 入站为这三种格式之一时可原样直连转发，无需格式归一化。
func IsMultiNativeProvider(providerType int) bool {
	switch ProviderType(providerType) {
	case ProviderNewAPI, ProviderSub2API:
		return true
	}
	return false
}

// HasNativeClaudeEndpoint 判断供应商是否另挂 Anthropic 兼容端点。
//
// 这类供应商主协议是 OpenAI（ProviderNativeFormat 返回 openai），但对
// RelayModeClaudeMessages 另有独立的 Anthropic 兼容端点（见各 adaptor 的
// GetRequestURL：/anthropic/v1/messages、/apps/anthropic/v1/messages、
// /api/coding/v1/messages 等），Claude 入站时上游**实际说 Claude**：
// 请求体须按 Claude 原样发出、响应体也是 Claude 格式。
//
// 因此协议格式不能只按渠道类型判定，必须叠加 relay mode——
// 漏掉这条会把 Claude 请求转成 OpenAI chat 发到 Anthropic 端点（上游 400），
// 并把返回的 Claude 响应当 OpenAI 解析（转换失败）。
//
// 新增此类供应商时同步登记：判据是 GetRequestURL 对 ClaudeMessages
// 返回了与 chat 端点**不同**的地址。若 ClaudeMessages 与 chat 共用同一端点
// （aws / baidu_v2 / mistral / openai / xai / xunfei），则上游只说 OpenAI，
// 不属于本集合，Claude 入站仍须转换。
func HasNativeClaudeEndpoint(providerType int) bool {
	switch ProviderType(providerType) {
	case ProviderAli, ProviderZhipu, ProviderDeepSeek,
		ProviderMoonshot, ProviderVolcengine, ProviderMiniMax:
		return true
	}
	return false
}
