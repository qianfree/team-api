package constant

// RelayFormat 定义入站请求和上游供应商的协议格式。
// 用于决定适配器是否需要进行格式转换。
type RelayFormat string

const (
	RelayFormatOpenAI    RelayFormat = "openai"    // OpenAI Chat Completions / Completions / Embeddings 等格式
	RelayFormatClaude    RelayFormat = "claude"    // Claude Messages API 格式
	RelayFormatGemini    RelayFormat = "gemini"    // Google Gemini API 格式
	RelayFormatResponses RelayFormat = "responses" // OpenAI Responses API 格式

	// 剩余原生格式供应商（非 OpenAI 兼容）。
	// 字符串值必须与 relaykit/types.RelayFormat 保持一致，桥接处依赖 types.RelayFormat(x) 强转。
	RelayFormatCoze   RelayFormat = "coze"   // 字节 Coze v3
	RelayFormatDify   RelayFormat = "dify"   // Dify chat-messages
	RelayFormatOllama RelayFormat = "ollama" // Ollama /api/chat
)
