package dto

// ==================== Claude Messages API ====================

// ClaudeRequest Claude Messages API 请求
type ClaudeRequest struct {
	Model         string          `json:"model"`
	System        any             `json:"system,omitempty"` // string 或 []ClaudeContentBlock
	Messages      []ClaudeMessage `json:"messages"`
	MaxTokens     *uint           `json:"max_tokens,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        *bool           `json:"stream,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	TopK          *int            `json:"top_k,omitempty"`
	Thinking      *ClaudeThinking `json:"thinking,omitempty"`
	Tools         []ClaudeTool    `json:"tools,omitempty"`
	ToolChoice    any             `json:"tool_choice,omitempty"`
	Metadata      any             `json:"metadata,omitempty"`
	ServiceTier   string          `json:"service_tier,omitempty"` // "auto" | "standard_only"
	Container     any             `json:"container,omitempty"`    // 容器沙箱配置
	McpServers    any             `json:"mcp_servers,omitempty"`  // MCP 服务器配置
}

// ClaudeThinking Claude 扩展思维配置
type ClaudeThinking struct {
	Type         string `json:"type"`
	BudgetTokens *int   `json:"budget_tokens,omitempty"`
}

// ClaudeMessage Claude 消息
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string 或 []ClaudeContentBlock
}

// ClaudeContentBlock Claude 内容块
type ClaudeContentBlock struct {
	Type         string              `json:"type"`
	Text         *string             `json:"text,omitempty"`
	Thinking     *string             `json:"thinking,omitempty"`
	Signature    string              `json:"signature,omitempty"`
	ID           string              `json:"id,omitempty"`
	Name         string              `json:"name,omitempty"`
	Input        any                 `json:"input,omitempty"`
	ToolUseID    string              `json:"tool_use_id,omitempty"`
	Content      any                 `json:"content,omitempty"`
	IsError      *bool               `json:"is_error,omitempty"` // tool_result 错误标志
	Source       *ClaudeSource       `json:"source,omitempty"`
	CacheControl *ClaudeCacheControl `json:"cache_control,omitempty"`
}

// ClaudeSource 多模态内容源
type ClaudeSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// ClaudeCacheControl 缓存控制
type ClaudeCacheControl struct {
	Type string `json:"type"`
}

// ClaudeTool Claude 工具定义
type ClaudeTool struct {
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	InputSchema  any                 `json:"input_schema,omitempty"`
	Type         string              `json:"type,omitempty"`          // "custom"（默认）或内置类型
	CacheControl *ClaudeCacheControl `json:"cache_control,omitempty"` // Prompt Caching 断点
}

// ClaudeToolChoice Claude 工具选择
type ClaudeToolChoice struct {
	Type                   string `json:"type"`
	Name                   string `json:"name,omitempty"`
	DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
}

// ClaudeResponse Claude Messages API 响应（流式和非流式共用）
type ClaudeResponse struct {
	ID           string               `json:"id,omitempty"`
	Type         string               `json:"type"` // message, message_start, content_block_start, content_block_delta, message_delta, message_stop
	Role         string               `json:"role,omitempty"`
	Content      []ClaudeContentBlock `json:"content,omitempty"`
	StopReason   string               `json:"stop_reason,omitempty"`
	StopSequence *string              `json:"stop_sequence,omitempty"`
	Model        string               `json:"model,omitempty"`
	Usage        *ClaudeUsage         `json:"usage,omitempty"`
	Index        *int                 `json:"index,omitempty"`
	ContentBlock *ClaudeContentBlock  `json:"content_block,omitempty"`
	Delta        *ClaudeDelta         `json:"delta,omitempty"`
	Message      *ClaudeMessageInfo   `json:"message,omitempty"`
	Error        any                  `json:"error,omitempty"`
	Container    any                  `json:"container,omitempty"`
}

// ClaudeDelta Claude 流式增量
type ClaudeDelta struct {
	Type         string  `json:"type,omitempty"`
	Text         *string `json:"text,omitempty"`
	PartialJSON  *string `json:"partial_json,omitempty"`
	StopReason   *string `json:"stop_reason,omitempty"`
	StopSequence *string `json:"stop_sequence,omitempty"`
	Thinking     *string `json:"thinking,omitempty"`
	Signature    string  `json:"signature,omitempty"`
}

// ClaudeMessageInfo Claude 消息元信息（message_start 事件中的 message 字段）
type ClaudeMessageInfo struct {
	ID           string               `json:"id"`
	Type         string               `json:"type"`
	Role         string               `json:"role"`
	Content      []ClaudeContentBlock `json:"content"`
	Model        string               `json:"model"`
	StopReason   *string              `json:"stop_reason"`
	StopSequence *string              `json:"stop_sequence"`
	Usage        *ClaudeUsage         `json:"usage"`
}

// ClaudeCacheUsage 缓存写入按 TTL 细分
type ClaudeCacheUsage struct {
	Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens,omitempty"`
	Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens,omitempty"`
}

// ClaudeServerToolUsage 服务器端内置工具使用统计
type ClaudeServerToolUsage struct {
	WebSearchRequests int `json:"web_search_requests,omitempty"`
}

// ClaudeUsage Claude 用量
type ClaudeUsage struct {
	InputTokens              int                    `json:"input_tokens"`
	OutputTokens             int                    `json:"output_tokens"`
	CacheCreationInputTokens int                    `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int                    `json:"cache_read_input_tokens"`
	CacheCreation            *ClaudeCacheUsage      `json:"cache_creation,omitempty"`
	ServerToolUse            *ClaudeServerToolUsage `json:"server_tool_use,omitempty"`
	ServiceTier              string                 `json:"service_tier,omitempty"`
}
