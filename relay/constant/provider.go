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
	ProviderCoze        ProviderType = 32
	ProviderDify        ProviderType = 33
	ProviderJimeng      ProviderType = 34
	ProviderCodex       ProviderType = 35
	ProviderSora        ProviderType = 37
	ProviderKling       ProviderType = 38
	ProviderSuno        ProviderType = 39
	ProviderMidjourney  ProviderType = 40
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
	case ProviderCoze:
		return "Coze"
	case ProviderDify:
		return "Dify"
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
	default:
		return "Unknown"
	}
}
