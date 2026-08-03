package types

// EndpointType 标识下游 API 接口。从 constant 包迁移至此，
// 这样转换工具包（dto/relayconvert）就不需要导入宿主包；
// constant 包中保留了供宿主代码使用的别名。
type EndpointType string

const (
	EndpointTypeOpenAI                EndpointType = "openai"
	EndpointTypeOpenAIResponse        EndpointType = "openai-response"
	EndpointTypeOpenAIResponseCompact EndpointType = "openai-response-compact"
	EndpointTypeOpenAIAlphaSearch     EndpointType = "openai-alpha-search"
	EndpointTypeAnthropic             EndpointType = "anthropic"
	EndpointTypeGemini                EndpointType = "gemini"
	EndpointTypeJinaRerank            EndpointType = "jina-rerank"
	EndpointTypeImageGeneration       EndpointType = "image-generation"
	EndpointTypeEmbeddings            EndpointType = "embeddings"
	EndpointTypeOpenAIVideo           EndpointType = "openai-video"
)

// OpenAI 兼容响应格式共享的结束原因。
// 声明为 vars（而非 consts），因为转换器代码会对它们取地址，
// 用于 *string 类型的 finish-reason 字段。
var (
	FinishReasonStop          = "stop"
	FinishReasonToolCalls     = "tool_calls"
	FinishReasonLength        = "length"
	FinishReasonFunctionCall  = "function_call"
	FinishReasonContentFilter = "content_filter"
)
