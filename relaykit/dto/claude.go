package dto

import (
	"encoding/json"
	"strings"
)

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

	// 服务端内置工具（web_search_* 等）的配置字段。重序列化（如模型名改写）时必须保留，
	// 否则搜索限制配置会静默丢失（wire 保真测试 dto/wire_fidelity_test.go 守护）。
	MaxUses        *int     `json:"max_uses,omitempty"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	BlockedDomains []string `json:"blocked_domains,omitempty"`
	UserLocation   any      `json:"user_location,omitempty"`
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

// ClaudeServerToolUsage 服务器端内置工具使用统计（厂商权威累计报数）。
// 字段命名规律固定为「工具名_requests」（web_search_requests / web_fetch_requests /
// code_execution_requests / 未来新增工具同构），故按动态 map 解析全部字段：
// 上游新增工具无需同步改 DTO，采集层按「去 _requests 后缀 = 计费工具名」泛化透出。
// 误采未知字段无害：计数与价格解耦，价格表未配置的工具不计费。
type ClaudeServerToolUsage struct {
	// Counts 工具名（已剥 _requests 后缀）→ 累计次数
	Counts map[string]int `json:",omitempty"`
}

// serverToolRequestsSuffix server_tool_use 计数字段的固定后缀
const serverToolRequestsSuffix = "_requests"

// UnmarshalJSON 动态解析：仅接受「工具名_requests」且值为正整数的字段，
// 其余字段/坏值逐个忽略（读侧容错，坏数据不阻断计费提取）。
// 剥后缀后的 key 即计费工具名，与结算层 ToolCalls key 同构。
func (s *ClaudeServerToolUsage) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	counts := make(map[string]int, len(raw))
	for key, v := range raw {
		tool, ok := strings.CutSuffix(key, serverToolRequestsSuffix)
		if !ok || tool == "" {
			continue
		}
		var n int
		if err := json.Unmarshal(v, &n); err != nil || n <= 0 {
			continue
		}
		counts[tool] = n
	}
	s.Counts = counts
	return nil
}

// MarshalJSON 对称还原：工具名加回 _requests 后缀（协议转换桥重组响应时保持 wire 形态）。
// 值接收者：ClaudeUsage.ServerToolUse 为指针时同样命中，且避免值拷贝经 interface
// 传递后因指针方法集不可寻址而静默回退到默认结构体序列化（字段名直出破坏 wire 形态）
func (s ClaudeServerToolUsage) MarshalJSON() ([]byte, error) {
	if len(s.Counts) == 0 {
		return []byte("null"), nil
	}
	raw := make(map[string]int, len(s.Counts))
	for tool, n := range s.Counts {
		raw[tool+serverToolRequestsSuffix] = n
	}
	return json.Marshal(raw)
}

// Count 指定工具的累计次数（未报数 / 零次 = 0）
func (s *ClaudeServerToolUsage) Count(tool string) int {
	if s == nil {
		return 0
	}
	return s.Counts[tool]
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
