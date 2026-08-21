package oai_chat

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

func TestOpenAIToClaudeRequestConverter_Metadata(t *testing.T) {
	converter := &OpenAIToClaudeRequestConverter{}

	if converter.ID() != relayconvert.ConverterOpenAIChatToClaudeMessages {
		t.Errorf("ID() = %q, want %q", converter.ID(), relayconvert.ConverterOpenAIChatToClaudeMessages)
	}

	if converter.From() != types.RelayFormatOpenAI {
		t.Errorf("From() = %q, want %q", converter.From(), types.RelayFormatOpenAI)
	}

	if converter.To() != types.RelayFormatClaude {
		t.Errorf("To() = %q, want %q", converter.To(), types.RelayFormatClaude)
	}

	if converter.Quality() != relayconvert.RequestConverterQualityFair {
		t.Errorf("Quality() = %q, want %q", converter.Quality(), relayconvert.RequestConverterQualityFair)
	}
}

func TestOpenAIToClaudeRequestConverter_BasicConversion(t *testing.T) {
	converter := &OpenAIToClaudeRequestConverter{}
	ctx := context.Background()

	maxTokens := 1024
	stream := true
	openaiReq := &dto.GeneralOpenAIRequest{
		Model:     "gpt-4",
		MaxTokens: &maxTokens,
		Stream:    &stream,
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "Hello, Claude!",
			},
		},
	}

	info := &mockMeta{
		upstreamModel: "claude-3-opus-20240229",
	}

	result, err := converter.ConvertRequest(ctx, info, openaiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	claudeReq, ok := result.(*dto.ClaudeRequest)
	if !ok {
		t.Fatalf("Expected *dto.ClaudeRequest, got %T", result)
	}

	if claudeReq.Model != "claude-3-opus-20240229" {
		t.Errorf("Model = %q, want %q", claudeReq.Model, "claude-3-opus-20240229")
	}

	if claudeReq.Stream == nil || !*claudeReq.Stream {
		t.Error("Expected Stream = true")
	}

	if claudeReq.MaxTokens == nil || *claudeReq.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %v, want 1024", claudeReq.MaxTokens)
	}

	if len(claudeReq.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(claudeReq.Messages))
	}

	if claudeReq.Messages[0].Role != "user" {
		t.Errorf("Message role = %q, want %q", claudeReq.Messages[0].Role, "user")
	}

	blocks, ok := claudeReq.Messages[0].Content.([]dto.ClaudeContentBlock)
	if !ok {
		t.Fatalf("Expected []ClaudeContentBlock, got %T", claudeReq.Messages[0].Content)
	}

	if len(blocks) != 1 || blocks[0].Type != "text" || *blocks[0].Text != "Hello, Claude!" {
		t.Errorf("Unexpected message content: %+v", blocks)
	}
}

func TestOpenAIToClaudeRequestConverter_SystemMessages(t *testing.T) {
	converter := &OpenAIToClaudeRequestConverter{}
	ctx := context.Background()

	maxTokens := 1024
	openaiReq := &dto.GeneralOpenAIRequest{
		Model:     "gpt-4",
		MaxTokens: &maxTokens,
		Messages: []dto.Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "system",
				Content: "Answer concisely.",
			},
			{
				Role:    "user",
				Content: "Hi",
			},
		},
	}

	info := &mockMeta{upstreamModel: "claude-3-opus-20240229"}

	result, err := converter.ConvertRequest(ctx, info, openaiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	claudeReq := result.(*dto.ClaudeRequest)

	expectedSystem := "You are a helpful assistant.\n\nAnswer concisely."
	if claudeReq.System != expectedSystem {
		t.Errorf("System = %q, want %q", claudeReq.System, expectedSystem)
	}

	if len(claudeReq.Messages) != 1 {
		t.Errorf("Expected 1 user message, got %d", len(claudeReq.Messages))
	}
}

func TestOpenAIToClaudeRequestConverter_ToolCalls(t *testing.T) {
	converter := &OpenAIToClaudeRequestConverter{}
	ctx := context.Background()

	maxTokens := 1024
	parameters := json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`)

	openaiReq := &dto.GeneralOpenAIRequest{
		Model:     "gpt-4",
		MaxTokens: &maxTokens,
		Tools: []dto.Tool{
			{
				Type: "function",
				Function: dto.FunctionDef{
					Name:        "get_weather",
					Description: "Get weather",
					Parameters:  parameters,
				},
			},
		},
		ToolChoice: "auto",
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "What's the weather?",
			},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []dto.ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Function: dto.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"city":"San Francisco"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				Content:    "Sunny, 72°F",
				ToolCallID: "call_123",
			},
		},
	}

	info := &mockMeta{upstreamModel: "claude-3-opus-20240229"}

	result, err := converter.ConvertRequest(ctx, info, openaiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	claudeReq := result.(*dto.ClaudeRequest)

	// 校验 tools
	if len(claudeReq.Tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(claudeReq.Tools))
	}

	if claudeReq.Tools[0].Name != "get_weather" {
		t.Errorf("Tool name = %q, want %q", claudeReq.Tools[0].Name, "get_weather")
	}

	// 校验 tool_choice
	toolChoice, ok := claudeReq.ToolChoice.(map[string]any)
	if !ok {
		t.Fatalf("Expected map[string]any for ToolChoice, got %T", claudeReq.ToolChoice)
	}

	if toolChoice["type"] != "auto" {
		t.Errorf("ToolChoice type = %q, want %q", toolChoice["type"], "auto")
	}

	// 校验 messages：user、带 tool_use 的 assistant、带 tool_result 的 user
	if len(claudeReq.Messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(claudeReq.Messages))
	}

	// assistant 消息应包含 tool_use block
	assistantBlocks, ok := claudeReq.Messages[1].Content.([]dto.ClaudeContentBlock)
	if !ok {
		t.Fatalf("Expected []ClaudeContentBlock for assistant, got %T", claudeReq.Messages[1].Content)
	}

	foundToolUse := false
	for _, block := range assistantBlocks {
		if block.Type == "tool_use" {
			foundToolUse = true
			if block.Name != "get_weather" {
				t.Errorf("Tool use name = %q, want %q", block.Name, "get_weather")
			}
		}
	}

	if !foundToolUse {
		t.Error("Expected tool_use block in assistant message")
	}

	// tool 结果消息的 role 应为 user，并包含 tool_result block
	if claudeReq.Messages[2].Role != "user" {
		t.Errorf("Tool result message role = %q, want %q", claudeReq.Messages[2].Role, "user")
	}

	toolResultBlocks, ok := claudeReq.Messages[2].Content.([]dto.ClaudeContentBlock)
	if !ok {
		t.Fatalf("Expected []ClaudeContentBlock for tool result, got %T", claudeReq.Messages[2].Content)
	}

	if len(toolResultBlocks) != 1 || toolResultBlocks[0].Type != "tool_result" {
		t.Errorf("Expected single tool_result block, got %+v", toolResultBlocks)
	}

	if toolResultBlocks[0].ToolUseID != "call_123" {
		t.Errorf("ToolUseID = %q, want %q", toolResultBlocks[0].ToolUseID, "call_123")
	}
}

func TestOpenAIToClaudeRequestConverter_ThinkingSuffix(t *testing.T) {
	converter := &OpenAIToClaudeRequestConverter{}
	ctx := context.Background()

	maxTokens := 2000
	openaiReq := &dto.GeneralOpenAIRequest{
		Model:     "gpt-4",
		MaxTokens: &maxTokens,
		Messages: []dto.Message{
			{Role: "user", Content: "Test"},
		},
	}

	tests := []struct {
		name               string
		upstreamModel      string
		adapterEnabled     bool
		budgetPercentage   float64
		expectThinking     bool
		expectThinkingType string // 无 budget 时为 adaptive（上游自适应），有 budget 时 enabled
		expectBudgetTokens bool
	}{
		{
			name:               "thinking suffix with adapter enabled",
			upstreamModel:      "claude-3-opus-20240229-thinking",
			adapterEnabled:     true,
			expectThinking:     true,
			expectThinkingType: "adaptive",
		},
		{
			name:               "thinking suffix with budget",
			upstreamModel:      "claude-3-opus-20240229-thinking",
			adapterEnabled:     true,
			budgetPercentage:   0.2,
			expectThinking:     true,
			expectThinkingType: "enabled",
			expectBudgetTokens: true,
		},
		{
			name:           "thinking suffix with adapter disabled",
			upstreamModel:  "claude-3-opus-20240229-thinking",
			adapterEnabled: false,
			expectThinking: false,
		},
		{
			name:           "nothinking suffix overrides",
			upstreamModel:  "claude-3-opus-20240229-nothinking",
			adapterEnabled: true,
			expectThinking: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &mockMeta{
				upstreamModel: tt.upstreamModel,
				opts: &convmeta.Options{
					Claude: convmeta.ClaudeOptions{
						ThinkingAdapterEnabled:                tt.adapterEnabled,
						ThinkingAdapterBudgetTokensPercentage: tt.budgetPercentage,
					},
				},
			}

			result, err := converter.ConvertRequest(ctx, info, openaiReq)
			if err != nil {
				t.Fatalf("ConvertRequest failed: %v", err)
			}

			claudeReq := result.(*dto.ClaudeRequest)

			if tt.expectThinking {
				if claudeReq.Thinking == nil {
					t.Error("Expected Thinking to be set, got nil")
				} else if tt.expectThinkingType != "" && claudeReq.Thinking.Type != tt.expectThinkingType {
					t.Errorf("Thinking.Type = %q, want %q", claudeReq.Thinking.Type, tt.expectThinkingType)
				}

				if tt.expectBudgetTokens {
					if claudeReq.Thinking.BudgetTokens == nil {
						t.Error("Expected BudgetTokens to be set, got nil")
					}
				}
			} else {
				if claudeReq.Thinking != nil {
					t.Errorf("Expected Thinking to be nil, got %+v", claudeReq.Thinking)
				}
			}
		})
	}
}

func TestOpenAIToClaudeRequestConverter_MaxTokensDefault(t *testing.T) {
	converter := &OpenAIToClaudeRequestConverter{}
	ctx := context.Background()

	openaiReq := &dto.GeneralOpenAIRequest{
		Model:     "gpt-4",
		MaxTokens: nil, // 不提供 max_tokens
		Messages: []dto.Message{
			{Role: "user", Content: "Test"},
		},
	}

	t.Run("with default available", func(t *testing.T) {
		info := &mockMeta{
			upstreamModel: "claude-3-opus-20240229",
			opts: &convmeta.Options{
				Claude: convmeta.ClaudeOptions{
					DefaultMaxTokens: func(modelName string) int {
						defaults := map[string]int{
							"claude-3-opus-20240229": 4096,
						}
						return defaults[modelName]
					},
				},
			},
		}

		result, err := converter.ConvertRequest(ctx, info, openaiReq)
		if err != nil {
			t.Fatalf("ConvertRequest failed: %v", err)
		}

		claudeReq := result.(*dto.ClaudeRequest)

		if claudeReq.MaxTokens == nil || *claudeReq.MaxTokens != 4096 {
			t.Errorf("MaxTokens = %v, want 4096", claudeReq.MaxTokens)
		}
	})

	t.Run("without default", func(t *testing.T) {
		info := &mockMeta{
			upstreamModel: "claude-3-opus-20240229",
		}

		_, err := converter.ConvertRequest(ctx, info, openaiReq)
		if err == nil {
			t.Error("Expected error when max_tokens not provided and no default available")
		}
	})
}

func TestOpenAIToClaudeRequestConverter_ToolChoiceVariants(t *testing.T) {
	converter := &OpenAIToClaudeRequestConverter{}
	ctx := context.Background()

	maxTokens := 1024
	parameters := json.RawMessage(`{"type":"object"}`)

	tests := []struct {
		name         string
		toolChoice   any
		expectedType string
		expectedName string
		expectNil    bool
	}{
		{
			name:         "auto",
			toolChoice:   "auto",
			expectedType: "auto",
		},
		{
			name:         "required",
			toolChoice:   "required",
			expectedType: "any",
		},
		{
			name:       "none",
			toolChoice: "none",
			expectNil:  true,
		},
		{
			name: "specific function",
			toolChoice: map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": "get_weather",
				},
			},
			expectedType: "tool",
			expectedName: "get_weather",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openaiReq := &dto.GeneralOpenAIRequest{
				Model:     "gpt-4",
				MaxTokens: &maxTokens,
				Tools: []dto.Tool{
					{
						Type: "function",
						Function: dto.FunctionDef{
							Name:       "get_weather",
							Parameters: parameters,
						},
					},
				},
				ToolChoice: tt.toolChoice,
				Messages: []dto.Message{
					{Role: "user", Content: "Test"},
				},
			}

			info := &mockMeta{upstreamModel: "claude-3-opus-20240229"}

			result, err := converter.ConvertRequest(ctx, info, openaiReq)
			if err != nil {
				t.Fatalf("ConvertRequest failed: %v", err)
			}

			claudeReq := result.(*dto.ClaudeRequest)

			if tt.expectNil {
				if claudeReq.ToolChoice != nil {
					t.Errorf("Expected ToolChoice to be nil, got %v", claudeReq.ToolChoice)
				}
				return
			}

			toolChoice, ok := claudeReq.ToolChoice.(map[string]any)
			if !ok {
				t.Fatalf("Expected map[string]any for ToolChoice, got %T", claudeReq.ToolChoice)
			}

			if toolChoice["type"] != tt.expectedType {
				t.Errorf("ToolChoice type = %q, want %q", toolChoice["type"], tt.expectedType)
			}

			if tt.expectedName != "" {
				if toolChoice["name"] != tt.expectedName {
					t.Errorf("ToolChoice name = %q, want %q", toolChoice["name"], tt.expectedName)
				}
			}
		})
	}
}

func TestOpenAIToClaudeRequestConverter_MultiModalContent(t *testing.T) {
	converter := &OpenAIToClaudeRequestConverter{}
	ctx := context.Background()

	maxTokens := 1024
	openaiReq := &dto.GeneralOpenAIRequest{
		Model:     "gpt-4-vision",
		MaxTokens: &maxTokens,
		Messages: []dto.Message{
			{
				Role: "user",
				Content: []dto.ContentPart{
					{
						Type: "text",
						Text: "What's in this image?",
					},
					{
						Type: "image_url",
						ImageURL: &dto.ImageURL{
							URL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAUA",
						},
					},
				},
			},
		},
	}

	info := &mockMeta{upstreamModel: "claude-3-opus-20240229"}

	result, err := converter.ConvertRequest(ctx, info, openaiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	claudeReq := result.(*dto.ClaudeRequest)

	if len(claudeReq.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(claudeReq.Messages))
	}

	blocks, ok := claudeReq.Messages[0].Content.([]dto.ClaudeContentBlock)
	if !ok {
		t.Fatalf("Expected []ClaudeContentBlock, got %T", claudeReq.Messages[0].Content)
	}

	if len(blocks) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(blocks))
	}

	if blocks[0].Type != "text" || *blocks[0].Text != "What's in this image?" {
		t.Errorf("First block: type=%q text=%q", blocks[0].Type, *blocks[0].Text)
	}

	if blocks[1].Type != "image" {
		t.Errorf("Second block type = %q, want %q", blocks[1].Type, "image")
	}

	if blocks[1].Source == nil {
		t.Error("Expected Source to be set for image block")
	}
}

// mockMeta 实现用于测试的 convmeta.Meta 接口
type mockMeta struct {
	upstreamModel string
	originModel   string
	opts          *convmeta.Options
}

func (m *mockMeta) GetUpstreamModelName() string {
	return m.upstreamModel
}

func (m *mockMeta) GetOriginModelName() string {
	if m.originModel != "" {
		return m.originModel
	}
	return "gpt-4"
}

func (m *mockMeta) GetOptions() *convmeta.Options {
	if m.opts != nil {
		return m.opts
	}
	return &convmeta.Options{}
}

func (m *mockMeta) HasChannelMeta() bool {
	return true
}

func (m *mockMeta) GetChannelID() int {
	return 0
}

func (m *mockMeta) GetChannelType() int {
	return 0
}

func (m *mockMeta) GetIsStream() bool {
	return false
}

func (m *mockMeta) GetReasoningEffort() string {
	return ""
}

func (m *mockMeta) SetReasoningEffort(effort string) {
}

func (m *mockMeta) GetEstimatePromptTokens() int {
	return 0
}

func (m *mockMeta) EnsureClaudeConvertInfo() *convmeta.ClaudeConvertInfo {
	return &convmeta.ClaudeConvertInfo{}
}

func (m *mockMeta) GetSendResponseCount() int {
	return 0
}

func (m *mockMeta) IncrSendResponseCount() {
}

func (m *mockMeta) AppendRequestConversion(format types.RelayFormat) {
}

func (m *mockMeta) ConvOptions() *convmeta.Options {
	return m.GetOptions()
}

// TestOpenAIToClaudeRequestConverter_ToolCallsNoEmptyTextBlock 回归：带 tool_calls 的
// assistant 消息 content 为 nil/""（codex 等 Responses 客户端的 function_call 历史经
// responses→chat 链转换后的形态）时，不得产生空 text 块——Anthropic 协议（含 GLM 的
// claude 兼容端点）对 "text":"" 直接 400 参数错误，导致工具调用第二轮请求被拒。
func TestOpenAIToClaudeRequestConverter_ToolCallsNoEmptyTextBlock(t *testing.T) {
	converter := &OpenAIToClaudeRequestConverter{}
	ctx := context.Background()
	maxTokens := 1024

	toolCall := dto.ToolCall{
		ID:   "call_456",
		Type: "function",
		Function: dto.FunctionCall{
			Name:      "shell",
			Arguments: `{"command":["ls","-la"]}`,
		},
	}

	for _, tc := range []struct {
		name    string
		content any
	}{
		{"nil content（responses 链输出形态）", nil},
		{"empty string content", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			openaiReq := &dto.GeneralOpenAIRequest{
				Model:     "gpt-4",
				MaxTokens: &maxTokens,
				Messages: []dto.Message{
					{Role: "user", Content: "list files"},
					{Role: "assistant", Content: tc.content, ToolCalls: []dto.ToolCall{toolCall}},
					{Role: "tool", ToolCallID: "call_456", Content: `{"stdout":"..."}`},
				},
			}
			result, err := converter.ConvertRequest(ctx, &mockMeta{upstreamModel: "claude-3-5-sonnet-20241022"}, openaiReq)
			if err != nil {
				t.Fatalf("ConvertRequest failed: %v", err)
			}
			claudeReq := result.(*dto.ClaudeRequest)

			blocks, ok := claudeReq.Messages[1].Content.([]dto.ClaudeContentBlock)
			if !ok {
				t.Fatalf("assistant content = %T, want []ClaudeContentBlock", claudeReq.Messages[1].Content)
			}
			if len(blocks) != 1 || blocks[0].Type != "tool_use" {
				t.Fatalf("assistant blocks = %+v, want 仅一个 tool_use 块（无空 text 块）", blocks)
			}
			// 整体 marshal 后不应出现空文本块
			raw, _ := json.Marshal(claudeReq)
			if bytes.Contains(raw, []byte(`"text":""`)) {
				t.Errorf("serialized request contains empty text block: %s", raw)
			}
		})
	}

	// 纯空消息（无文本无工具调用）：兜底单个空格 text 块，不产生空 content
	openaiReq := &dto.GeneralOpenAIRequest{
		Model:     "gpt-4",
		MaxTokens: &maxTokens,
		Messages: []dto.Message{
			{Role: "assistant", Content: ""},
		},
	}
	result, err := converter.ConvertRequest(ctx, &mockMeta{upstreamModel: "claude-3-5-sonnet-20241022"}, openaiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}
	blocks := result.(*dto.ClaudeRequest).Messages[0].Content.([]dto.ClaudeContentBlock)
	if len(blocks) != 1 || blocks[0].Type != "text" || (blocks[0].Text == nil || *blocks[0].Text != " ") {
		t.Errorf("empty assistant message blocks = %+v, want single space text block", blocks)
	}
}
