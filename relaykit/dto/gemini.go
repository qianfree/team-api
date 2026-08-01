package dto

import "encoding/json"

// ==================== Gemini API ====================

// GeminiChatRequest Gemini Chat 请求
type GeminiChatRequest struct {
	Contents          []GeminiContent         `json:"contents"`
	SafetySettings    []GeminiSafetySetting   `json:"safetySettings,omitempty"`
	GenerationConfig  *GeminiGenerationConfig `json:"generationConfig,omitempty"`
	Tools             json.RawMessage         `json:"tools,omitempty"`
	ToolConfig        any                     `json:"toolConfig,omitempty"`
	SystemInstruction *GeminiContent          `json:"systemInstruction,omitempty"`
	CachedContent     string                  `json:"cachedContent,omitempty"`
	ServiceTier       string                  `json:"serviceTier,omitempty"`
	Store             *bool                   `json:"store,omitempty"`
}

// GeminiContent Gemini 内容（角色 + 部分）
type GeminiContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart Gemini 内容部分
type GeminiPart struct {
	Text                string                     `json:"text,omitempty"`
	InlineData          *GeminiInlineData          `json:"inlineData,omitempty"`
	FileData            *GeminiFileData            `json:"fileData,omitempty"`
	FunctionCall        *GeminiFunctionCall        `json:"functionCall,omitempty"`
	FunctionResponse    *GeminiFunctionResponse    `json:"functionResponse,omitempty"`
	Thought             *bool                      `json:"thought,omitempty"`
	ThoughtSignature    string                     `json:"thoughtSignature,omitempty"`
	ExecutableCode      *GeminiExecutableCode      `json:"executableCode,omitempty"`
	CodeExecutionResult *GeminiCodeExecutionResult `json:"codeExecutionResult,omitempty"`
	VideoMetadata       *GeminiVideoMetadata       `json:"videoMetadata,omitempty"`
}

// GeminiInlineData Gemini 内联数据（图片、音频等）
type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// GeminiFileData Gemini 文件引用数据
type GeminiFileData struct {
	MimeType string `json:"mimeType,omitempty"`
	FileURI  string `json:"fileUri,omitempty"`
}

// GeminiFunctionCall Gemini 函数调用
type GeminiFunctionCall struct {
	ID           string `json:"id,omitempty"`
	FunctionName string `json:"name"`
	Arguments    any    `json:"args,omitempty"`
}

// GeminiFunctionResponse Gemini 函数响应
type GeminiFunctionResponse struct {
	ID           string `json:"id,omitempty"`
	Name         string `json:"name"`
	Response     any    `json:"response,omitempty"`
	WillContinue bool   `json:"willContinue,omitempty"`
	Scheduling   string `json:"scheduling,omitempty"`
}

// GeminiExecutableCode Gemini 可执行代码
type GeminiExecutableCode struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

// GeminiCodeExecutionResult Gemini 代码执行结果
type GeminiCodeExecutionResult struct {
	Outcome string `json:"outcome"`
	Output  string `json:"output"`
}

// GeminiVideoMetadata Gemini 视频元数据
type GeminiVideoMetadata struct {
	StartOffset string `json:"startOffset,omitempty"`
	EndOffset   string `json:"endOffset,omitempty"`
}

// GeminiGenerationConfig Gemini 生成配置
type GeminiGenerationConfig struct {
	Temperature        *float64              `json:"temperature,omitempty"`
	TopP               *float64              `json:"topP,omitempty"`
	TopK               *float64              `json:"topK,omitempty"`
	MaxOutputTokens    *uint                 `json:"maxOutputTokens,omitempty"`
	CandidateCount     *int                  `json:"candidateCount,omitempty"`
	StopSequences      []string              `json:"stopSequences,omitempty"`
	ResponseMimeType   string                `json:"responseMimeType,omitempty"`
	ResponseSchema     any                   `json:"responseSchema,omitempty"`
	ResponseJsonSchema any                   `json:"responseJsonSchema,omitempty"`
	ResponseModalities []string              `json:"responseModalities,omitempty"`
	PresencePenalty    *float64              `json:"presencePenalty,omitempty"`
	FrequencyPenalty   *float64              `json:"frequencyPenalty,omitempty"`
	Seed               *int64                `json:"seed,omitempty"`
	ResponseLogprobs   *bool                 `json:"responseLogprobs,omitempty"`
	Logprobs           *int                  `json:"logprobs,omitempty"`
	ThinkingConfig     *GeminiThinkingConfig `json:"thinkingConfig,omitempty"`
	SpeechConfig       any                   `json:"speechConfig,omitempty"`
	MediaResolution    string                `json:"mediaResolution,omitempty"`
	ImageConfig        *GeminiImageConfig    `json:"imageConfig,omitempty"`
}

// GeminiImageConfig Banana 内生图配置（用于 Chat 模式的图片生成模型）
type GeminiImageConfig struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
	ImageSize   string `json:"imageSize,omitempty"`
}

// GeminiThinkingConfig Gemini 思考配置
type GeminiThinkingConfig struct {
	IncludeThoughts bool   `json:"includeThoughts"`
	ThoughtBudget   *int   `json:"thoughtBudget,omitempty"`
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`
}

// GeminiSafetySetting Gemini 安全设置
type GeminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// GeminiChatResponse Gemini Chat 响应
type GeminiChatResponse struct {
	Candidates     []GeminiCandidate     `json:"candidates,omitempty"`
	PromptFeedback *GeminiPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *GeminiUsageMetadata  `json:"usageMetadata,omitempty"`
	ModelName      string                `json:"modelVersion,omitempty"`
	ResponseID     string                `json:"responseId,omitempty"`
}

// GeminiCandidate Gemini 候选结果
type GeminiCandidate struct {
	Content           *GeminiContent `json:"content,omitempty"`
	FinishReason      string         `json:"finishReason,omitempty"`
	FinishMessage     string         `json:"finishMessage,omitempty"`
	Index             int            `json:"index,omitempty"`
	SafetyRatings     []any          `json:"safetyRatings,omitempty"`
	CitationMetadata  any            `json:"citationMetadata,omitempty"`
	GroundingMetadata any            `json:"groundingMetadata,omitempty"`
	TokenCount        int            `json:"tokenCount,omitempty"`
	AvgLogprobs       *float64       `json:"avgLogprobs,omitempty"`
	LogprobsResult    any            `json:"logprobsResult,omitempty"`
}

// GeminiPromptFeedback Gemini 提示反馈
type GeminiPromptFeedback struct {
	SafetyRatings []any  `json:"safetyRatings,omitempty"`
	BlockReason   string `json:"blockReason,omitempty"`
}

// GeminiUsageMetadata Gemini 用量元数据
type GeminiUsageMetadata struct {
	PromptTokenCount           int                        `json:"promptTokenCount"`
	CandidatesTokenCount       int                        `json:"candidatesTokenCount"`
	TotalTokenCount            int                        `json:"totalTokenCount"`
	CachedContentTokenCount    int                        `json:"cachedContentTokenCount,omitempty"`
	ThoughtsTokenCount         int                        `json:"thoughtsTokenCount,omitempty"`
	ToolUsePromptTokenCount    int                        `json:"toolUsePromptTokenCount,omitempty"`
	PromptTokensDetails        []GeminiModalityTokenCount `json:"promptTokensDetails,omitempty"`
	CandidatesTokensDetails    []GeminiModalityTokenCount `json:"candidatesTokensDetails,omitempty"`
	ToolUsePromptTokensDetails []GeminiModalityTokenCount `json:"toolUsePromptTokensDetails,omitempty"`
	CacheTokensDetails         []GeminiModalityTokenCount `json:"cacheTokensDetails,omitempty"`
}

// GeminiModalityTokenCount 按模态的 Token 计数
type GeminiModalityTokenCount struct {
	Modality   string `json:"modality"`
	TokenCount int    `json:"tokenCount"`
}

// GeminiFunctionDeclaration Gemini 函数声明（用于工具定义序列化）
type GeminiFunctionDeclaration struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// GeminiModel Gemini 模型信息（/v1beta/models 响应）
type GeminiModel struct {
	Name                       string   `json:"name"`
	BaseModelId                string   `json:"baseModelId,omitempty"`
	Version                    string   `json:"version,omitempty"`
	DisplayName                string   `json:"displayName,omitempty"`
	Description                string   `json:"description,omitempty"`
	InputTokenLimit            int      `json:"inputTokenLimit,omitempty"`
	OutputTokenLimit           int      `json:"outputTokenLimit,omitempty"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods,omitempty"`
	Temperature                *float64 `json:"temperature,omitempty"`
	MaxTemperature             *float64 `json:"maxTemperature,omitempty"`
	TopP                       *float64 `json:"topP,omitempty"`
	TopK                       *int     `json:"topK,omitempty"`
}

// GeminiModelsResponse Gemini 模型列表响应
type GeminiModelsResponse struct {
	Models        []GeminiModel `json:"models"`
	NextPageToken string        `json:"nextPageToken,omitempty"`
}

// ==================== Gemini Imagen Image API ====================

// GeminiImageRequest Imagen 图像生成请求
type GeminiImageRequest struct {
	Instances  []GeminiImageInstance `json:"instances"`
	Parameters GeminiImageParameters `json:"parameters"`
}

// GeminiImageInstance Imagen 图像生成实例
type GeminiImageInstance struct {
	Prompt string `json:"prompt"`
}

// GeminiImageParameters Imagen 图像生成参数
type GeminiImageParameters struct {
	SampleCount      int    `json:"sampleCount,omitempty"`
	AspectRatio      string `json:"aspectRatio,omitempty"`
	PersonGeneration string `json:"personGeneration,omitempty"`
	ImageSize        string `json:"imageSize,omitempty"`
}

// GeminiImageResponse Imagen 图像生成响应
type GeminiImageResponse struct {
	Predictions []GeminiImagePrediction `json:"predictions"`
}

// GeminiImagePrediction Imagen 图像生成预测结果
type GeminiImagePrediction struct {
	MimeType           string `json:"mimeType"`
	BytesBase64Encoded string `json:"bytesBase64Encoded"`
	RaiFilteredReason  string `json:"raiFilteredReason,omitempty"`
	SafetyAttributes   any    `json:"safetyAttributes,omitempty"`
}
