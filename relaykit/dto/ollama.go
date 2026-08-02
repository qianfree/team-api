// Package dto — Ollama /api/chat 协议数据结构。
//
// 从 relay/channel/ollama 提取的纯数据结构，供 relaykit 转换器与宿主桥接层使用。
// 仅覆盖 chat 路径（RelayModeChatCompletions）；generate（completions）与 embedding
// 的 DTO 仍保留在 relay/channel/ollama，本阶段不迁移。
// 不依赖任何宿主类型（RelayInfo / GoFrame），仅含协议字段。
package dto

// OllamaChatRequest Ollama Chat 请求（/api/chat）。
type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options,omitempty"`
}

// OllamaMessage Ollama 消息格式。Content 为纯文本（多模态图片经 base64 放入 Images）。
type OllamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

// OllamaChatResponse Ollama Chat 响应（流式与非流式共用）。
//
// 流式（NDJSON）每行一个本结构；非流式为单个本结构。
// 最后一条（Done==true）携带 PromptEvalCount / EvalCount 用量。
type OllamaChatResponse struct {
	Model           string        `json:"model"`
	CreatedAt       string        `json:"created_at"`
	Message         OllamaMessage `json:"message"`
	Done            bool          `json:"done"`
	TotalDuration   int64         `json:"total_duration,omitempty"`
	PromptEvalCount int           `json:"prompt_eval_count,omitempty"`
	EvalCount       int           `json:"eval_count,omitempty"`
}
