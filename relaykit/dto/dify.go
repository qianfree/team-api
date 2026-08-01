// Package dto — Dify chat-messages 协议数据结构。
//
// 阶段 5 从 relay/channel/dify 提取的纯数据结构，供 relaykit 转换器与宿主桥接层使用。
// 不依赖任何宿主类型（RelayInfo / GoFrame），仅含协议字段。
package dto

// DifyRequest Dify Chat Messages 请求体。
// 所有 OpenAI 消息被拼接为单个 Query 字符串；ResponseMode 由是否流式决定。
type DifyRequest struct {
	Inputs       map[string]any `json:"inputs"`
	Query        string                 `json:"query"`
	ResponseMode string                 `json:"response_mode"` // "blocking" | "streaming"
	User         string                 `json:"user"`
}

// DifyUsage Dify 响应中的 token 用量。
type DifyUsage struct {
	TotalTokens      int `json:"total_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// DifyBlockingResponse Dify 非流式（blocking）响应。
type DifyBlockingResponse struct {
	Answer         string    `json:"answer"`
	Metadata       DifyMeta  `json:"metadata"`
	ConversationID string    `json:"conversation_id"`
	MessageID      string    `json:"message_id"`
}

// DifyMeta Dify 响应 metadata，承载用量信息。
type DifyMeta struct {
	Usage DifyUsage `json:"usage"`
}

// DifyStreamEvent Dify 流式 SSE 事件数据（每条 data 行的载荷）。
//
// 事件类型（Event 字段）：
//   - "message"：正文增量（Answer 字段为本次增量文本）
//   - "message_end"：流结束，Metadata.Usage 携带用量
//   - "error"：错误
type DifyStreamEvent struct {
	Event          string    `json:"event"`
	Answer         string    `json:"answer"`
	ConversationID string    `json:"conversation_id"`
	MessageID      string    `json:"message_id"`
	Metadata       DifyMeta  `json:"metadata"`
}
