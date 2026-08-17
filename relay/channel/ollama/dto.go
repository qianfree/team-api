package ollama

import relaykitdto "github.com/qianfree/team-api/relaykit/dto"

// chat 路径的 3 个 DTO 与 relaykit/dto 字节相同，别名到 relaykit 统一权威定义。
type OllamaChatRequest = relaykitdto.OllamaChatRequest
type OllamaMessage = relaykitdto.OllamaMessage
type OllamaChatResponse = relaykitdto.OllamaChatResponse
type OllamaTool = relaykitdto.OllamaTool
type OllamaToolFunction = relaykitdto.OllamaToolFunction
type OllamaToolCall = relaykitdto.OllamaToolCall

// OllamaGenerateRequest Ollama Generate（文本补全）请求
type OllamaGenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options,omitempty"`
}

// OllamaEmbeddingRequest Ollama Embedding 请求
type OllamaEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// OllamaGenerateResponse Ollama Generate 响应（流式和非流式共用）
type OllamaGenerateResponse struct {
	Model           string `json:"model"`
	CreatedAt       string `json:"created_at"`
	Response        string `json:"response"`
	Done            bool   `json:"done"`
	TotalDuration   int64  `json:"total_duration,omitempty"`
	PromptEvalCount int    `json:"prompt_eval_count,omitempty"`
	EvalCount       int    `json:"eval_count,omitempty"`
}

// OllamaEmbeddingResponse Ollama Embedding 响应
type OllamaEmbeddingResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float64 `json:"embeddings"`
}
