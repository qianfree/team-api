package dto

// Usage Token 使用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// UsageWithDetails 带 token 细分的 Usage（用于详细计费）
type UsageWithDetails struct {
	PromptTokens           int           `json:"prompt_tokens"`
	CompletionTokens       int           `json:"completion_tokens"`
	TotalTokens            int           `json:"total_tokens"`
	PromptTokensDetails    *TokenDetails `json:"prompt_tokens_details,omitempty"`
	CompletionTokenDetails *TokenDetails `json:"completion_tokens_details,omitempty"`
}

// TokenDetails Token 类型细分
type TokenDetails struct {
	CachedTokens             int `json:"cached_tokens,omitempty"`
	CachedCreationTokens     int `json:"cached_creation_tokens,omitempty"`    // Claude cache_creation_input_tokens
	CachedCreation5mTokens   int `json:"cached_creation_5m_tokens,omitempty"` // Claude 5分钟缓存创建
	CachedCreation1hTokens   int `json:"cached_creation_1h_tokens,omitempty"` // Claude 1小时缓存创建
	AudioTokens              int `json:"audio_tokens,omitempty"`
	TextTokens               int `json:"text_tokens,omitempty"`
	ImageTokens              int `json:"image_tokens,omitempty"`
	ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty"`
}

// CompletionsRequest 文本补全请求
type CompletionsRequest struct {
	Model            string         `json:"model"`
	Prompt           any            `json:"prompt"` // string 或 []string
	MaxTokens        *int           `json:"max_tokens,omitempty"`
	Temperature      *float64       `json:"temperature,omitempty"`
	TopP             *float64       `json:"top_p,omitempty"`
	N                *int           `json:"n,omitempty"`
	Stream           *bool          `json:"stream,omitempty"`
	StreamOptions    *StreamOptions `json:"stream_options,omitempty"`
	Stop             any            `json:"stop,omitempty"`
	Suffix           string         `json:"suffix,omitempty"`
	User             string         `json:"user,omitempty"`
	Echo             *bool          `json:"echo,omitempty"`
	LogProbs         *bool          `json:"logprobs,omitempty"`
	TopLogProbs      *int           `json:"top_logprobs,omitempty"`
	LogitBias        map[string]int `json:"logit_bias,omitempty"`
	PresencePenalty  *float64       `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64       `json:"frequency_penalty,omitempty"`
	BestOf           *int           `json:"best_of,omitempty"`
}

// CompletionsResponse 文本补全响应
type CompletionsResponse struct {
	ID                string              `json:"id"`
	Object            string              `json:"object"`
	Created           int64               `json:"created"`
	Model             string              `json:"model"`
	SystemFingerprint *string             `json:"system_fingerprint,omitempty"`
	Choices           []CompletionsChoice `json:"choices"`
	Usage             UsageWithDetails    `json:"usage"`
}

// CompletionsChoice 补全选择项
type CompletionsChoice struct {
	Index        int    `json:"index"`
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason"`
	LogProbs     any    `json:"logprobs,omitempty"`
}

// CompletionsStreamResponse 补全流式响应
type CompletionsStreamResponse struct {
	ID                string                    `json:"id"`
	Object            string                    `json:"object"`
	Created           int64                     `json:"created"`
	Model             string                    `json:"model"`
	SystemFingerprint *string                   `json:"system_fingerprint,omitempty"`
	Choices           []CompletionsStreamChoice `json:"choices"`
	Usage             *UsageWithDetails         `json:"usage"`
}

// CompletionsStreamChoice 补全流式选择项
type CompletionsStreamChoice struct {
	Index        int     `json:"index"`
	Text         string  `json:"text"`
	FinishReason *string `json:"finish_reason"`
}

// EmbeddingRequest 嵌入请求
type EmbeddingRequest struct {
	Model          string `json:"model"`
	Input          any    `json:"input"` // string 或 []string
	EncodingFormat string `json:"encoding_format,omitempty"`
	Dimensions     *int   `json:"dimensions,omitempty"`
	User           string `json:"user,omitempty"`
}

// EmbeddingResponse 嵌入响应
type EmbeddingResponse struct {
	Object string           `json:"object"`
	Data   []Embedding      `json:"data"`
	Model  string           `json:"model"`
	Usage  UsageWithDetails `json:"usage"`
}

// Embedding 单个嵌入结果
type Embedding struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// ImageRequest 图像生成请求
type ImageRequest struct {
	Model             string `json:"model"`
	Prompt            string `json:"prompt"`
	N                 *int   `json:"n,omitempty"`
	Size              string `json:"size,omitempty"`
	Quality           string `json:"quality,omitempty"`
	ResponseFormat    string `json:"response_format,omitempty"`
	Style             string `json:"style,omitempty"`
	User              string `json:"user,omitempty"`
	OutputFormat      string `json:"output_format,omitempty"`
	OutputCompression *int   `json:"output_compression,omitempty"`
	Background        string `json:"background,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
	InputImageMask    any    `json:"input_image_mask,omitempty"`
}

// ImageResponse 图像生成响应
type ImageResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
}

// ImageData 单个图像数据
type ImageData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	ContentType   string `json:"content_type,omitempty"`
}
