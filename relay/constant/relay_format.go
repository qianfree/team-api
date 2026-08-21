package constant

import relaykittypes "github.com/qianfree/team-api/relaykit/types"

// RelayFormat 定义入站请求和上游供应商的协议格式。
// 类型别名到 relaykit/types 的权威定义，消除两边字符串值的人工同步约定
// （此前桥接层依赖 types.RelayFormat(x) 强转，值不一致会导致路由查不到）。
type RelayFormat = relaykittypes.RelayFormat

const (
	RelayFormatOpenAI    = relaykittypes.RelayFormatOpenAI          // OpenAI Chat Completions / Completions / Embeddings 等格式
	RelayFormatClaude    = relaykittypes.RelayFormatClaude          // Claude Messages API 格式
	RelayFormatGemini    = relaykittypes.RelayFormatGemini          // Google Gemini API 格式
	RelayFormatResponses = relaykittypes.RelayFormatOpenAIResponses // OpenAI Responses API 格式（relaykit 侧名为 OpenAIResponses，常量名保留宿主习惯）

	// 剩余原生格式供应商（非 OpenAI 兼容）。
	RelayFormatCoze   = relaykittypes.RelayFormatCoze   // 字节 Coze v3
	RelayFormatDify   = relaykittypes.RelayFormatDify   // Dify chat-messages
	RelayFormatOllama = relaykittypes.RelayFormatOllama // Ollama /api/chat
)
