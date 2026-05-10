package dto

import "encoding/json"

// GeneralOpenAIRequest OpenAI 兼容的 Chat Completions 请求
type GeneralOpenAIRequest struct {
	Model                string          `json:"model"`
	Messages             []Message       `json:"messages"`
	MaxTokens            *int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens  *int            `json:"max_completion_tokens,omitempty"`
	Temperature          *float64        `json:"temperature,omitempty"`
	TopP                 *float64        `json:"top_p,omitempty"`
	TopK                 *int            `json:"top_k,omitempty"`
	N                    *int            `json:"n,omitempty"`
	Stream               *bool           `json:"stream,omitempty"`
	StreamOptions        *StreamOptions  `json:"stream_options,omitempty"`
	Stop                 any             `json:"stop,omitempty"`
	PresencePenalty      *float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty     *float64        `json:"frequency_penalty,omitempty"`
	User                 string          `json:"user,omitempty"`
	Seed                 *int64          `json:"seed,omitempty"`
	Tools                []Tool          `json:"tools,omitempty"`
	ToolChoice           any             `json:"tool_choice,omitempty"`
	ParallelToolCalls    *bool           `json:"parallel_tool_calls,omitempty"`
	ResponseFormat       *ResponseFormat `json:"response_format,omitempty"`
	LogProbs             *bool           `json:"logprobs,omitempty"`
	TopLogProbs          *int            `json:"top_logprobs,omitempty"`
	ReasoningEffort      string          `json:"reasoning_effort,omitempty"`
	LogitBias            map[string]int  `json:"logit_bias,omitempty"`
	ServiceTier          string          `json:"service_tier,omitempty"`
	Modalities           []string        `json:"modalities,omitempty"`
	Audio                json.RawMessage `json:"audio,omitempty"`
	Store                *bool           `json:"store,omitempty"`
	WebSearchOptions     json.RawMessage `json:"web_search_options,omitempty"`
	Prediction           json.RawMessage `json:"prediction,omitempty"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
	SafetyIdentifier     string          `json:"safety_identifier,omitempty"`
	Verbosity            string          `json:"verbosity,omitempty"`
	PromptCacheKey       string          `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string          `json:"prompt_cache_retention,omitempty"`
	ImageConfig          *ImageConfigDTO `json:"image_config,omitempty"`
}

// ImageConfigDTO OpenAI 请求中的图片生成配置透传字段（用于 Banana 等内生图模型）
type ImageConfigDTO struct {
	AspectRatio string `json:"aspect_ratio,omitempty"`
	ImageSize   string `json:"image_size,omitempty"`
}

// Message OpenAI 消息格式
type Message struct {
	Role             string     `json:"role"`
	Content          any        `json:"content"`                     // string 或 []ContentPart
	Refusal          *string    `json:"refusal,omitempty"`           // 结构化输出拒绝原因
	ReasoningContent *string    `json:"reasoning_content,omitempty"` // 推理模型（DeepSeek/o1）的思考内容
	Name             string     `json:"name,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Annotations      []any      `json:"annotations,omitempty"`
}

// ContentPart 多模态内容块
type ContentPart struct {
	Type       string      `json:"type"`
	Text       string      `json:"text,omitempty"`
	ImageURL   *ImageURL   `json:"image_url,omitempty"`
	InputAudio *InputAudio `json:"input_audio,omitempty"`
	File       *FileData   `json:"file,omitempty"`
}

// ImageURL 图片 URL
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// InputAudio 音频输入
type InputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

// FileData 文件附件
type FileData struct {
	FileID   string `json:"file_id,omitempty"`
	FileData string `json:"file_data,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// Tool 工具定义
type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef 函数定义
type FunctionDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
	Strict      *bool  `json:"strict,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	Index    int          `json:"index,omitempty"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// StreamOptions 流式选项
type StreamOptions struct {
	IncludeUsage       bool `json:"include_usage"`
	IncludeObfuscation bool `json:"include_obfuscation,omitempty"`
}

// ResponseFormat 响应格式
type ResponseFormat struct {
	Type       string `json:"type"`
	JSONSchema any    `json:"json_schema,omitempty"`
}

// ChatCompletionResponse Chat Completions 非流式响应
type ChatCompletionResponse struct {
	ID                string           `json:"id"`
	Object            string           `json:"object"`
	Created           int64            `json:"created"`
	Model             string           `json:"model"`
	SystemFingerprint *string          `json:"system_fingerprint,omitempty"`
	Choices           []Choice         `json:"choices"`
	Usage             UsageWithDetails `json:"usage"`
	ServiceTier       *string          `json:"service_tier,omitempty"`
}

// Choice 响应选择项
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
	LogProbs     any     `json:"logprobs,omitempty"`
}

// ChatCompletionStreamResponse Chat Completions 流式响应块
type ChatCompletionStreamResponse struct {
	ID                string            `json:"id"`
	Object            string            `json:"object"`
	Created           int64             `json:"created"`
	Model             string            `json:"model"`
	SystemFingerprint *string           `json:"system_fingerprint,omitempty"`
	Choices           []StreamChoice    `json:"choices"`
	Usage             *UsageWithDetails `json:"usage"`
}

// StreamChoice 流式选择项
type StreamChoice struct {
	Index        int     `json:"index"`
	Delta        Message `json:"delta"`
	FinishReason *string `json:"finish_reason"`
	LogProbs     any     `json:"logprobs,omitempty"`
}

// ModelsResponse /v1/models 响应
type ModelsResponse struct {
	Object string     `json:"object"`
	Data   []ModelDTO `json:"data"`
}

// ModelDTO 模型信息 DTO
type ModelDTO struct {
	ID              string           `json:"id"`
	Object          string           `json:"object"`
	Created         int64            `json:"created"`
	OwnedBy         string           `json:"owned_by"`
	ModelName       string           `json:"model_name,omitempty"`
	Category        string           `json:"category,omitempty"`
	ContextWindow   int              `json:"context_window,omitempty"`
	MaxOutputTokens int              `json:"max_output_tokens,omitempty"`
	Capabilities    map[string]bool  `json:"capabilities,omitempty"`
	Modalities      *ModelModalities `json:"modalities,omitempty"`
}

// ModelDetailResponse /v1/models/{model_id} 响应
type ModelDetailResponse struct {
	ID              string           `json:"id"`
	Object          string           `json:"object"`
	Created         int64            `json:"created"`
	OwnedBy         string           `json:"owned_by"`
	ModelName       string           `json:"model_name,omitempty"`
	Description     string           `json:"description,omitempty"`
	Category        string           `json:"category,omitempty"`
	Status          string           `json:"status,omitempty"`
	ContextWindow   int              `json:"context_window,omitempty"`
	MaxOutputTokens int              `json:"max_output_tokens,omitempty"`
	Capabilities    map[string]bool  `json:"capabilities,omitempty"`
	Modalities      *ModelModalities `json:"modalities,omitempty"`
	Deprecated      bool             `json:"deprecated"`
}

// ModelModalities 模型输入输出模态
type ModelModalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}
