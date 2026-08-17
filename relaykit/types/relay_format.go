package types

type RelayFormat string

const (
	RelayFormatOpenAI RelayFormat = "openai"
	RelayFormatClaude             = "claude"
	RelayFormatGemini             = "gemini"
	// RelayFormatOpenAIResponses 字符串值与宿主 relay/constant.RelayFormatResponses 保持一致：
	// 桥接层依赖 types.RelayFormat(x) 直接强转（字符串值相等约定），值不一致会导致路由查不到。
	RelayFormatOpenAIResponses           = "responses"
	RelayFormatOpenAIResponsesCompaction = "openai_responses_compaction"
	RelayFormatOpenAIAlphaSearch         = "openai_alpha_search"
	RelayFormatOpenAIAudio               = "openai_audio"
	RelayFormatOpenAIImage               = "openai_image"
	RelayFormatOpenAIRealtime            = "openai_realtime"
	RelayFormatRerank                    = "rerank"
	RelayFormatEmbedding                 = "embedding"

	RelayFormatTask    = "task"
	RelayFormatMjProxy = "mj_proxy"

	// 剩余原生格式供应商（非 OpenAI 兼容）
	RelayFormatCoze   = "coze"   // 字节 Coze v3
	RelayFormatDify   = "dify"   // Dify chat-messages
	RelayFormatOllama = "ollama" // Ollama /api/chat
)
