package dto

import "encoding/json"

// ========== Responses API 请求 ==========

// OpenAIResponsesRequest OpenAI Responses API 请求
// https://platform.openai.com/docs/api-reference/responses/create
type OpenAIResponsesRequest struct {
	Model                string          `json:"model"`
	Input                json.RawMessage `json:"input,omitempty"`
	Include              json.RawMessage `json:"include,omitempty"`
	Instructions         json.RawMessage `json:"instructions,omitempty"`
	MaxOutputTokens      *uint           `json:"max_output_tokens,omitempty"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
	ParallelToolCalls    json.RawMessage `json:"parallel_tool_calls,omitempty"`
	PreviousResponseID   string          `json:"previous_response_id,omitempty"`
	Reasoning            *Reasoning      `json:"reasoning,omitempty"`
	Store                json.RawMessage `json:"store,omitempty"`
	Stream               *bool           `json:"stream,omitempty"`
	StreamOptions        *StreamOptions  `json:"stream_options,omitempty"`
	Temperature          *float64        `json:"temperature,omitempty"`
	Text                 json.RawMessage `json:"text,omitempty"`
	ToolChoice           json.RawMessage `json:"tool_choice,omitempty"`
	Tools                json.RawMessage `json:"tools,omitempty"`
	TopP                 *float64        `json:"top_p,omitempty"`
	Logprobs             *int            `json:"logprobs,omitempty"`
	TopLogProbs          *int            `json:"top_logprobs,omitempty"`
	Truncation           json.RawMessage `json:"truncation,omitempty"`
	User                 json.RawMessage `json:"user,omitempty"`
	MaxToolCalls         *uint           `json:"max_tool_calls,omitempty"`
	Prompt               json.RawMessage `json:"prompt,omitempty"`
	ServiceTier          string          `json:"service_tier,omitempty"`
	Conversation         json.RawMessage `json:"conversation,omitempty"`
	ContextManagement    json.RawMessage `json:"context_management,omitempty"`
	Background           *bool           `json:"background,omitempty"`
	PromptCacheKey       string          `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string          `json:"prompt_cache_retention,omitempty"`
	SafetyIdentifier     string          `json:"safety_identifier,omitempty"`
}

// Reasoning 推理参数（用于 o1/o3 等推理模型）
type Reasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// ========== Responses API 响应 ==========

// OpenAIResponsesResponse Responses API 非流式响应
type OpenAIResponsesResponse struct {
	ID                   string              `json:"id"`
	Object               string              `json:"object"`
	CreatedAt            int                 `json:"created_at"`
	CompletedAt          int                 `json:"completed_at,omitempty"`
	Status               json.RawMessage     `json:"status"`
	Error                any                 `json:"error"`
	IncompleteDetails    any                 `json:"incomplete_details"`
	Instructions         any                 `json:"instructions"`
	MaxOutputTokens      *int                `json:"max_output_tokens"`
	MaxToolCalls         *int                `json:"max_tool_calls,omitempty"`
	Model                string              `json:"model"`
	Output               []ResponsesOutput   `json:"output"`
	OutputText           string              `json:"output_text,omitempty"`
	ParallelToolCalls    bool                `json:"parallel_tool_calls"`
	PreviousResponseID   any                 `json:"previous_response_id"`
	Prompt               any                 `json:"prompt"`
	PromptCacheKey       string              `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string              `json:"prompt_cache_retention,omitempty"`
	Reasoning            *ResponsesReasoning `json:"reasoning"`
	SafetyIdentifier     string              `json:"safety_identifier,omitempty"`
	ServiceTier          string              `json:"service_tier,omitempty"`
	Store                bool                `json:"store"`
	Background           *bool               `json:"background,omitempty"`
	Conversation         any                 `json:"conversation"`
	Temperature          *float64            `json:"temperature"`
	Text                 *ResponsesText      `json:"text,omitempty"`
	ToolChoice           any                 `json:"tool_choice"`
	Tools                []any               `json:"tools"`
	TopP                 *float64            `json:"top_p"`
	Truncation           any                 `json:"truncation"`
	Usage                *ResponsesUsage     `json:"usage,omitempty"`
	User                 any                 `json:"user"`
	Metadata             any                 `json:"metadata"`
}

// IncompleteDetails 不完整响应详情
type IncompleteDetails struct {
	Reason string `json:"reason,omitempty"`
}

// ResponsesUsage Responses API 使用量（字段名与 Chat Completions 不同）
type ResponsesUsage struct {
	InputTokens        int                 `json:"input_tokens"`
	OutputTokens       int                 `json:"output_tokens"`
	TotalTokens        int                 `json:"total_tokens"`
	InputTokensDetails *InputTokenDetails  `json:"input_tokens_details"`
	OutputTokenDetails *OutputTokenDetails `json:"output_tokens_details"`
}

// ResponsesReasoning 推理配置（响应中的 reasoning 字段）
type ResponsesReasoning struct {
	Effort  any `json:"effort"`
	Summary any `json:"summary"`
}

// ResponsesText 文本配置（响应中的 text 字段）
type ResponsesText struct {
	Format ResponsesTextFormat `json:"format"`
}

// ResponsesTextFormat 文本格式
type ResponsesTextFormat struct {
	Type string `json:"type"`
}

// InputTokenDetails 输入 token 细分
type InputTokenDetails struct {
	CachedTokens int `json:"cached_tokens"`
	TextTokens   int `json:"text_tokens,omitempty"`
	AudioTokens  int `json:"audio_tokens,omitempty"`
	ImageTokens  int `json:"image_tokens,omitempty"`
}

// OutputTokenDetails 输出 token 细分
type OutputTokenDetails struct {
	TextTokens               int `json:"text_tokens,omitempty"`
	AudioTokens              int `json:"audio_tokens,omitempty"`
	ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty"`
}

// ResponsesOutput 响应输出项（通用结构，支持所有 output item 类型）
// 具体 type 包括：message、function_call、web_search_call、file_search_call、
// computer_call、reasoning、image_generation_call、code_interpreter_call、
// shell_call、local_shell_call、apply_patch_call、mcp_call、mcp_list_tools、
// mcp_approval_request、custom_tool_call、compaction
type ResponsesOutput struct {
	Type      string                   `json:"type"`
	ID        string                   `json:"id"`
	Status    string                   `json:"status,omitempty"`
	Role      string                   `json:"role,omitempty"`
	Content   []ResponsesOutputContent `json:"content,omitempty"`
	CallID    string                   `json:"call_id,omitempty"`
	Name      string                   `json:"name,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
	// web_search_call
	Action *ResponsesWebSearchAction `json:"action,omitempty"`
	// file_search_call
	Queries []string                    `json:"queries,omitempty"`
	Results []ResponsesFileSearchResult `json:"results,omitempty"`
	// computer_call
	PendingSafetyChecks []ResponsesSafetyCheck `json:"pending_safety_checks,omitempty"`
	// reasoning
	Summary          []ResponsesSummaryPart `json:"summary,omitempty"`
	EncryptedContent string                 `json:"encrypted_content,omitempty"`
	// image_generation_call
	Result string `json:"result,omitempty"`
	// code_interpreter_call
	Code        string                           `json:"code,omitempty"`
	ContainerID string                           `json:"container_id,omitempty"`
	Outputs     []ResponsesCodeInterpreterOutput `json:"outputs,omitempty"`
	// shell_call / local_shell_call
	ShellAction *ResponsesShellAction `json:"action,omitempty"`
	// apply_patch_call
	PatchAction *ResponsesPatchAction `json:"action,omitempty"`
	// mcp_call
	ServerLabel       string `json:"server_label,omitempty"`
	Output            string `json:"output,omitempty"`
	Error             string `json:"error,omitempty"`
	ApprovalRequestID string `json:"approval_request_id,omitempty"`
	// mcp_list_tools
	Tools []ResponsesMCPTool `json:"tools,omitempty"`
	// mcp_approval_request
	// custom_tool_call
	Input string `json:"input,omitempty"`
}

// ResponsesOutputContent 响应输出内容块
type ResponsesOutputContent struct {
	Type        string                `json:"type"`
	Text        string                `json:"text,omitempty"`
	Annotations []ResponsesAnnotation `json:"annotations,omitempty"`
	// refusal 内容类型
	Refusal string `json:"refusal,omitempty"`
	// reasoning_text 内容类型
}

// ResponsesAnnotation 注解类型（联合结构，支持所有注解子类型）
type ResponsesAnnotation struct {
	Type     string `json:"type"`
	FileID   string `json:"file_id,omitempty"`
	Filename string `json:"filename,omitempty"`
	Index    int    `json:"index,omitempty"`
	// url_citation
	URL        string `json:"url,omitempty"`
	Title      string `json:"title,omitempty"`
	StartIndex int    `json:"start_index,omitempty"`
	EndIndex   int    `json:"end_index,omitempty"`
}

// ResponsesWebSearchAction Web 搜索动作
type ResponsesWebSearchAction struct {
	Type    string                     `json:"type"`
	Query   string                     `json:"query,omitempty"`
	URL     string                     `json:"url,omitempty"`
	Pattern string                     `json:"pattern,omitempty"`
	Sources []ResponsesWebSearchSource `json:"sources,omitempty"`
}

// ResponsesWebSearchSource Web 搜索来源
type ResponsesWebSearchSource struct {
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
}

// ResponsesFileSearchResult 文件搜索结果项
type ResponsesFileSearchResult struct {
	FileID string  `json:"file_id"`
	Text   string  `json:"text"`
	Score  float64 `json:"score"`
}

// ResponsesSafetyCheck 安全检查项
type ResponsesSafetyCheck struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ResponsesCodeInterpreterOutput 代码解释器输出
type ResponsesCodeInterpreterOutput struct {
	Type string `json:"type"`
	Logs string `json:"logs,omitempty"`
	URL  string `json:"url,omitempty"`
}

// ResponsesShellAction Shell 命令动作
type ResponsesShellAction struct {
	Type             string            `json:"type"`
	Command          []string          `json:"command,omitempty"`
	MaxOutputLength  *int              `json:"max_output_length,omitempty"`
	Timeout          *int              `json:"timeout,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	User             string            `json:"user,omitempty"`
	WorkingDirectory string            `json:"working_directory,omitempty"`
}

// ResponsesPatchAction 补丁动作
type ResponsesPatchAction struct {
	Type  string `json:"type"`
	Path  string `json:"path,omitempty"`
	Patch string `json:"patch,omitempty"`
}

// ResponsesMCPTool MCP 工具描述
type ResponsesMCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema,omitempty"`
	Annotations any    `json:"annotations,omitempty"`
}

// ========== Responses API 流式响应 ==========

// ResponsesStreamResponse 流式事件包装
type ResponsesStreamResponse struct {
	Type         string                   `json:"type"`
	Response     *OpenAIResponsesResponse `json:"response,omitempty"`
	Delta        string                   `json:"delta,omitempty"`
	Item         *ResponsesOutput         `json:"item,omitempty"`
	OutputIndex  *int                     `json:"output_index,omitempty"`
	ContentIndex *int                     `json:"content_index,omitempty"`
	SummaryIndex *int                     `json:"summary_index,omitempty"`
	ItemID       string                   `json:"item_id,omitempty"`
	Part         *ResponsesSummaryPart    `json:"part,omitempty"`
}

// ResponsesSummaryPart 推理摘要部分
type ResponsesSummaryPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// HasError 检查响应是否包含错误
func (r *OpenAIResponsesResponse) HasError() bool {
	if r.Error == nil {
		return false
	}
	return true
}
