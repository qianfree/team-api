package types

type RelayFormat string

const (
	RelayFormatOpenAI RelayFormat = "openai"
	RelayFormatClaude             = "claude"
	RelayFormatGemini             = "gemini"
	// RelayFormatOpenAIResponses 宿主侧常量名为 relay/constant.RelayFormatResponses；
	// 宿主已通过类型别名直接引用本包定义，两侧为同一类型，无字符串值同步约定。
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
