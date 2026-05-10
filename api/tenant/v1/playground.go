package v1

import "github.com/gogf/gf/v2/frame/g"

// ============================================================
// Playground（真实调用，产生费用）
// ============================================================

// PlaygroundImageConfig Playground 图片生成配置
type PlaygroundImageConfig struct {
	AspectRatio string `json:"aspect_ratio,omitempty" dc:"宽高比：1:1, 16:9, 3:4, 4:3 等"`
	ImageSize   string `json:"image_size,omitempty" dc:"分辨率：512, 1K, 2K, 4K"`
}

// PlaygroundChatReq Playground 对话请求
type PlaygroundChatReq struct {
	g.Meta           `path:"/playground/chat" method:"post" mime:"json" tags:"租户控制台-Playground" summary:"Playground对话"`
	Model            string                 `json:"model" v:"required#请选择模型" dc:"模型名称"`
	Messages         []PlaygroundMessage    `json:"messages" v:"required#请输入消息" dc:"对话消息列表"`
	Temperature      *float64               `json:"temperature" dc:"温度参数 (0-2)"`
	MaxTokens        *int                   `json:"max_tokens" dc:"最大输出 Token 数"`
	TopP             *float64               `json:"top_p" dc:"Top-P 采样参数"`
	FrequencyPenalty *float64               `json:"frequency_penalty" dc:"频率惩罚 (-2 ~ 2)"`
	PresencePenalty  *float64               `json:"presence_penalty" dc:"存在惩罚 (-2 ~ 2)"`
	Stream           bool                   `json:"stream" d:"true" dc:"是否流式响应"`
	ImageConfig      *PlaygroundImageConfig `json:"image_config,omitempty" dc:"图片生成配置（仅图片模型）"`
}

type PlaygroundChatRes struct {
	Model            string `json:"model"`
	Content          string `json:"content" dc:"响应内容（非流式时返回）"`
	PromptTokens     int    `json:"prompt_tokens" dc:"输入 Token 数"`
	CompletionTokens int    `json:"completion_tokens" dc:"输出 Token 数"`
	TotalTokens      int    `json:"total_tokens" dc:"总 Token 数"`
	EstimatedCost    string `json:"estimated_cost" dc:"预估费用"`
}

type PlaygroundMessage struct {
	Role    string `json:"role" v:"required|in:system,user,assistant#请指定角色|角色无效"`
	Content string `json:"content" v:"required#消息内容不能为空"`
}

// ============================================================
// Playground — 图片生成
// ============================================================

type PlaygroundImageReq struct {
	g.Meta  `path:"/playground/image" method:"post" mime:"json" tags:"租户控制台-Playground" summary:"Playground图片生成"`
	Model   string `json:"model" v:"required#请选择模型" dc:"模型名称"`
	Prompt  string `json:"prompt" v:"required#请输入提示词" dc:"图片生成提示词"`
	N       *int   `json:"n,omitempty" dc:"生成数量 (1-4)"`
	Size    string `json:"size,omitempty" dc:"图片尺寸：1024x1024, 1792x1024, 1024x1792 等"`
	Quality string `json:"quality,omitempty" dc:"图片质量：standard, hd"`
}

type PlaygroundImageRes struct {
	Images           []PlaygroundImageData `json:"images"`
	PromptTokens     int                   `json:"prompt_tokens"`
	CompletionTokens int                   `json:"completion_tokens"`
	TotalTokens      int                   `json:"total_tokens"`
	EstimatedCost    string                `json:"estimated_cost"`
}

type PlaygroundImageData struct {
	B64JSON       string `json:"b64_json,omitempty"`
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// ============================================================
// Playground — 语音合成 (TTS)
// ============================================================

type PlaygroundAudioTTSReq struct {
	g.Meta         `path:"/playground/audio/tts" method:"post" mime:"json" tags:"租户控制台-Playground" summary:"Playground语音合成"`
	Model          string `json:"model" v:"required#请选择模型" dc:"模型名称"`
	Input          string `json:"input" v:"required#请输入文本" dc:"待合成文本"`
	Voice          string `json:"voice" v:"required#请选择语音" dc:"语音类型：alloy, echo, fable, onyx, nova, shimmer"`
	ResponseFormat string `json:"response_format,omitempty" d:"mp3" dc:"输出格式：mp3, wav, opus, aac, flac"`
}

type PlaygroundAudioTTSRes struct {
	AudioBase64      string `json:"audio_base64"`
	ContentType      string `json:"content_type"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	EstimatedCost    string `json:"estimated_cost"`
}

// ============================================================
// Playground — 文本嵌入
// ============================================================

type PlaygroundEmbeddingReq struct {
	g.Meta     `path:"/playground/embedding" method:"post" mime:"json" tags:"租户控制台-Playground" summary:"Playground文本嵌入"`
	Model      string `json:"model" v:"required#请选择模型" dc:"模型名称"`
	Input      string `json:"input" v:"required#请输入文本" dc:"待嵌入文本"`
	Dimensions *int   `json:"dimensions,omitempty" dc:"嵌入维度"`
}

type PlaygroundEmbeddingRes struct {
	Embeddings    []PlaygroundEmbeddingData `json:"embeddings"`
	PromptTokens  int                       `json:"prompt_tokens"`
	TotalTokens   int                       `json:"total_tokens"`
	EstimatedCost string                    `json:"estimated_cost"`
}

type PlaygroundEmbeddingData struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// ============================================================
// Playground — 重排序
// ============================================================

type PlaygroundRerankReq struct {
	g.Meta    `path:"/playground/rerank" method:"post" mime:"json" tags:"租户控制台-Playground" summary:"Playground重排序"`
	Model     string   `json:"model" v:"required#请选择模型" dc:"模型名称"`
	Query     string   `json:"query" v:"required#请输入查询" dc:"查询文本"`
	Documents []string `json:"documents" v:"required|length:1,100#请输入文档|文档数量超出限制" dc:"待排序文档列表"`
	TopN      *int     `json:"top_n,omitempty" dc:"返回前 N 个结果"`
}

type PlaygroundRerankRes struct {
	Results       []PlaygroundRerankResult `json:"results"`
	PromptTokens  int                      `json:"prompt_tokens"`
	TotalTokens   int                      `json:"total_tokens"`
	EstimatedCost string                   `json:"estimated_cost"`
}

type PlaygroundRerankResult struct {
	Index          int                  `json:"index"`
	RelevanceScore float64              `json:"relevance_score"`
	Document       *PlaygroundRerankDoc `json:"document,omitempty"`
}

type PlaygroundRerankDoc struct {
	Text string `json:"text"`
}

// ============================================================
// Sandbox（模拟调用，不计费）
// ============================================================

// SandboxChatReq Sandbox 对话请求
type SandboxChatReq struct {
	g.Meta      `path:"/sandbox/chat" method:"post" mime:"json" tags:"租户控制台-Playground" summary:"Sandbox模拟对话"`
	Model       string              `json:"model" v:"required#请选择模型" dc:"模型名称"`
	Messages    []PlaygroundMessage `json:"messages" v:"required|length:1,50#请输入消息|消息数量超出限制" dc:"对话消息列表"`
	Temperature *float64            `json:"temperature" dc:"温度参数 (0-2)"`
	MaxTokens   *int                `json:"max_tokens" dc:"最大输出 Token 数"`
	Stream      bool                `json:"stream" d:"true" dc:"是否流式响应"`
}

type SandboxChatRes struct {
	Content        string `json:"content"`
	IsSandbox      bool   `json:"is_sandbox"`
	RemainingQuota int    `json:"remaining_quota" dc:"本月剩余沙箱额度"`
}

// SandboxQuotaReq 沙箱额度查询
type SandboxQuotaReq struct {
	g.Meta `path:"/sandbox/quota" method:"get" tags:"租户控制台-Playground" summary:"查询沙箱额度"`
}

type SandboxQuotaRes struct {
	TotalQuota     int `json:"total_quota"`
	RemainingQuota int `json:"remaining_quota"`
	UsedQuota      int `json:"used_quota"`
}
