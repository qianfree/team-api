package oai_chat

import (
	"context"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// newClaudeInboundMeta 构造 Claude 入站方向测试用的 Meta（模型名已映射：origin != upstream）。
func newClaudeInboundMeta() *convmeta.Values {
	return &convmeta.Values{
		OriginModelName:     "claude-sonnet-4",
		UpstreamModelName:   "gpt-4o",
		ChannelMetaAttached: true,
	}
}

func TestClaudeToOpenAIRequestConverter_Metadata(t *testing.T) {
	converter := &ClaudeToOpenAIRequestConverter{}

	if converter.ID() != relayconvert.ConverterClaudeMessagesToOpenAIChat {
		t.Errorf("ID() = %q, want %q", converter.ID(), relayconvert.ConverterClaudeMessagesToOpenAIChat)
	}
	if converter.From() != types.RelayFormatClaude {
		t.Errorf("From() = %q, want %q", converter.From(), types.RelayFormatClaude)
	}
	if converter.To() != types.RelayFormatOpenAI {
		t.Errorf("To() = %q, want %q", converter.To(), types.RelayFormatOpenAI)
	}
	if converter.Quality() != relayconvert.RequestConverterQualityFair {
		t.Errorf("Quality() = %q, want %q", converter.Quality(), relayconvert.RequestConverterQualityFair)
	}
}

func TestClaudeToOpenAIRequestConverter_InvalidType(t *testing.T) {
	converter := &ClaudeToOpenAIRequestConverter{}
	if _, err := converter.ConvertRequest(context.Background(), newClaudeInboundMeta(), "not a request"); err == nil {
		t.Fatal("expected type assertion error, got nil")
	}
}

func TestClaudeToOpenAIRequestConverter_BasicConversion(t *testing.T) {
	converter := &ClaudeToOpenAIRequestConverter{}
	maxTokens := uint(2000)
	temperature := 0.7
	topP := 0.9
	stream := true

	claudeReq := &dto.ClaudeRequest{
		Model:         "claude-sonnet-4",
		MaxTokens:     &maxTokens,
		Temperature:   &temperature,
		TopP:          &topP,
		Stream:        &stream,
		StopSequences: []string{"STOP"},
		System:        "you are helpful",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
		},
	}

	result, err := converter.ConvertRequest(context.Background(), newClaudeInboundMeta(), claudeReq)
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	openaiReq, ok := result.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("expected *dto.GeneralOpenAIRequest, got %T", result)
	}

	// 模型名取上游模型名
	if openaiReq.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", openaiReq.Model)
	}
	if openaiReq.MaxTokens == nil || *openaiReq.MaxTokens != 2000 {
		t.Errorf("MaxTokens = %v, want 2000", openaiReq.MaxTokens)
	}
	if openaiReq.Temperature == nil || *openaiReq.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", openaiReq.Temperature)
	}
	if openaiReq.TopP == nil || *openaiReq.TopP != 0.9 {
		t.Errorf("TopP = %v, want 0.9", openaiReq.TopP)
	}
	if openaiReq.Stream == nil || !*openaiReq.Stream {
		t.Errorf("Stream = %v, want true", openaiReq.Stream)
	}
	stop, ok := openaiReq.Stop.([]string)
	if !ok || len(stop) != 1 || stop[0] != "STOP" {
		t.Errorf("Stop = %v, want [STOP]", openaiReq.Stop)
	}

	// system 消息 + user + assistant
	if len(openaiReq.Messages) != 3 {
		t.Fatalf("got %d messages, want 3", len(openaiReq.Messages))
	}
	if openaiReq.Messages[0].Role != "system" || openaiReq.Messages[0].Content != "you are helpful" {
		t.Errorf("system message = %+v", openaiReq.Messages[0])
	}
	if openaiReq.Messages[1].Role != "user" || openaiReq.Messages[1].Content != "hello" {
		t.Errorf("user message = %+v", openaiReq.Messages[1])
	}
	if openaiReq.Messages[2].Role != "assistant" || openaiReq.Messages[2].Content != "hi there" {
		t.Errorf("assistant message = %+v", openaiReq.Messages[2])
	}
}

// TestClaudeToOpenAIRequestConverter_MaxTokensDefault 未携带 max_tokens 时默认 4096。
func TestClaudeToOpenAIRequestConverter_MaxTokensDefault(t *testing.T) {
	converter := &ClaudeToOpenAIRequestConverter{}
	claudeReq := &dto.ClaudeRequest{
		Model:    "claude-sonnet-4",
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
	}

	result, err := converter.ConvertRequest(context.Background(), newClaudeInboundMeta(), claudeReq)
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	openaiReq := result.(*dto.GeneralOpenAIRequest)
	if openaiReq.MaxTokens == nil || *openaiReq.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %v, want default 4096", openaiReq.MaxTokens)
	}
}

// TestClaudeToOpenAIRequestConverter_Thinking thinking 启用时映射 reasoning_effort，
// 未启用（type != enabled）时不设置。
func TestClaudeToOpenAIRequestConverter_Thinking(t *testing.T) {
	converter := &ClaudeToOpenAIRequestConverter{}
	budget := 20000
	claudeReq := &dto.ClaudeRequest{
		Model:    "claude-sonnet-4",
		Thinking: &dto.ClaudeThinking{Type: "enabled", BudgetTokens: &budget},
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
	}

	result, err := converter.ConvertRequest(context.Background(), newClaudeInboundMeta(), claudeReq)
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	if got := result.(*dto.GeneralOpenAIRequest).ReasoningEffort; got != "high" {
		t.Errorf("ReasoningEffort = %q, want high", got)
	}

	claudeReq.Thinking = &dto.ClaudeThinking{Type: "disabled"}
	result, err = converter.ConvertRequest(context.Background(), newClaudeInboundMeta(), claudeReq)
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	if got := result.(*dto.GeneralOpenAIRequest).ReasoningEffort; got != "" {
		t.Errorf("ReasoningEffort = %q, want empty (thinking disabled)", got)
	}
}

// TestClaudeToOpenAIRequestConverter_Tools 工具定义与 tool_choice 转换。
func TestClaudeToOpenAIRequestConverter_Tools(t *testing.T) {
	converter := &ClaudeToOpenAIRequestConverter{}
	claudeReq := &dto.ClaudeRequest{
		Model: "claude-sonnet-4",
		Tools: []dto.ClaudeTool{{
			Name:        "get_weather",
			Description: "查询天气",
			InputSchema: map[string]any{"type": "object"},
		}},
		ToolChoice: map[string]any{"type": "tool", "name": "get_weather"},
		Messages:   []dto.ClaudeMessage{{Role: "user", Content: "天气"}},
	}

	result, err := converter.ConvertRequest(context.Background(), newClaudeInboundMeta(), claudeReq)
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	openaiReq := result.(*dto.GeneralOpenAIRequest)

	if len(openaiReq.Tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(openaiReq.Tools))
	}
	tool := openaiReq.Tools[0]
	if tool.Type != "function" || tool.Function.Name != "get_weather" || tool.Function.Description != "查询天气" {
		t.Errorf("tool = %+v", tool)
	}

	tc, ok := openaiReq.ToolChoice.(map[string]any)
	if !ok || tc["type"] != "function" {
		t.Fatalf("ToolChoice = %v, want function map", openaiReq.ToolChoice)
	}
	fn, ok := tc["function"].(map[string]any)
	if !ok || fn["name"] != "get_weather" {
		t.Errorf("ToolChoice.function = %v, want name=get_weather", tc["function"])
	}
}

func TestExtractClaudeSystemText(t *testing.T) {
	t.Run("plain string", func(t *testing.T) {
		if got := extractClaudeSystemText("you are helpful"); got != "you are helpful" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("array of text blocks joined", func(t *testing.T) {
		sys := []any{
			map[string]any{"type": "text", "text": "line1"},
			map[string]any{"type": "text", "text": "line2"},
		}
		if got := extractClaudeSystemText(sys); got != "line1\nline2" {
			t.Errorf("got %q, want \"line1\\nline2\"", got)
		}
	})
	t.Run("non-text blocks ignored", func(t *testing.T) {
		sys := []any{
			map[string]any{"type": "text", "text": "keep"},
			map[string]any{"type": "image", "text": "drop"},
		}
		if got := extractClaudeSystemText(sys); got != "keep" {
			t.Errorf("got %q, want \"keep\"", got)
		}
	})
	t.Run("unknown type returns empty", func(t *testing.T) {
		if got := extractClaudeSystemText(42); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestC2oJoinParts(t *testing.T) {
	tests := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"only"}, "only"},
		{[]string{"a", "b", "c"}, "a\nb\nc"},
	}
	for _, tt := range tests {
		if got := c2oJoinParts(tt.in); got != tt.want {
			t.Errorf("c2oJoinParts(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestC2oConvertThinkingToReasoningEffort(t *testing.T) {
	tests := []struct {
		name   string
		budget *int
		want   string
	}{
		{"nil budget defaults medium", nil, "medium"},
		{"low boundary", ptrInt(2048), "low"},
		{"low", ptrInt(1000), "low"},
		{"medium boundary", ptrInt(16384), "medium"},
		{"medium", ptrInt(8000), "medium"},
		{"high", ptrInt(16385), "high"},
		{"high large", ptrInt(50000), "high"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c2oConvertThinkingToReasoningEffort(&dto.ClaudeThinking{BudgetTokens: tt.budget})
			if got != tt.want {
				t.Errorf("budget=%v => %q, want %q", tt.budget, got, tt.want)
			}
		})
	}
}

func TestC2oConvertToolChoice(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := c2oConvertToolChoice(nil); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("string passthrough", func(t *testing.T) {
		if got := c2oConvertToolChoice("auto"); got != "auto" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("type mappings", func(t *testing.T) {
		cases := map[string]string{"auto": "auto", "any": "required", "none": "none"}
		for in, want := range cases {
			got := c2oConvertToolChoice(map[string]any{"type": in})
			if got != want {
				t.Errorf("type=%q => %v, want %q", in, got, want)
			}
		}
	})
	t.Run("specific tool maps to function", func(t *testing.T) {
		got := c2oConvertToolChoice(map[string]any{"type": "tool", "name": "get_weather"})
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", got)
		}
		if m["type"] != "function" {
			t.Errorf("type = %v, want function", m["type"])
		}
		fn, ok := m["function"].(map[string]any)
		if !ok || fn["name"] != "get_weather" {
			t.Errorf("function = %v, want name=get_weather", m["function"])
		}
	})
	t.Run("tool without name falls back to required", func(t *testing.T) {
		if got := c2oConvertToolChoice(map[string]any{"type": "tool"}); got != "required" {
			t.Errorf("got %v, want required", got)
		}
	})
}

func TestConvertClaudeUserMessage_StringContent(t *testing.T) {
	msgs := convertClaudeUserMessage(dto.ClaudeMessage{Role: "user", Content: "hi there"})
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hi there" {
		t.Errorf("got %+v", msgs[0])
	}
}

// TestConvertClaudeUserMessage_ToolResultArrayContent tool_result.content 为内容块数组
// （Claude 规范形式 [{type:"text",text:"..."}]）时，文本块须拼接保留，
// 否则工具结果会被替换为空字符串导致内容丢失。
func TestConvertClaudeUserMessage_ToolResultArrayContent(t *testing.T) {
	msgs := convertClaudeUserMessage(dto.ClaudeMessage{
		Role: "user",
		Content: []any{
			map[string]any{
				"type":        "tool_result",
				"tool_use_id": "toolu_123",
				"content": []any{
					map[string]any{"type": "text", "text": "result-a"},
					map[string]any{"type": "text", "text": "result-b"},
				},
			},
		},
	})
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].Role != "tool" {
		t.Errorf("role = %q, want tool", msgs[0].Role)
	}
	if msgs[0].ToolCallID != "toolu_123" {
		t.Errorf("ToolCallID = %q, want toolu_123", msgs[0].ToolCallID)
	}
	if msgs[0].Content != "result-a\nresult-b" {
		t.Errorf("content = %q, want %q", msgs[0].Content, "result-a\nresult-b")
	}
}

// TestConvertClaudeUserMessage_TextBeforeToolResult 文本块在 tool_result 之前时，
// 先输出 user 文本消息再输出 tool 消息。
func TestConvertClaudeUserMessage_TextBeforeToolResult(t *testing.T) {
	msgs := convertClaudeUserMessage(dto.ClaudeMessage{
		Role: "user",
		Content: []any{
			map[string]any{"type": "text", "text": "before"},
			map[string]any{"type": "tool_result", "tool_use_id": "toolu_1", "content": "result"},
		},
	})
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "before" {
		t.Errorf("msg[0] = %+v", msgs[0])
	}
	if msgs[1].Role != "tool" || msgs[1].Content != "result" || msgs[1].ToolCallID != "toolu_1" {
		t.Errorf("msg[1] = %+v", msgs[1])
	}
}

// TestConvertClaudeUserMessage_ImageContent base64 图片转换为 image_url 内容块。
func TestConvertClaudeUserMessage_ImageContent(t *testing.T) {
	msgs := convertClaudeUserMessage(dto.ClaudeMessage{
		Role: "user",
		Content: []any{
			map[string]any{"type": "text", "text": "看这张图"},
			map[string]any{
				"type":   "image",
				"source": map[string]any{"type": "base64", "media_type": "image/png", "data": "abc123"},
			},
		},
	})
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	parts, ok := msgs[0].Content.([]dto.ContentPart)
	if !ok || len(parts) != 2 {
		t.Fatalf("content = %+v, want 2 content parts", msgs[0].Content)
	}
	if parts[0].Type != "text" || parts[0].Text != "看这张图" {
		t.Errorf("parts[0] = %+v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil ||
		parts[1].ImageURL.URL != "data:image/png;base64,abc123" || parts[1].ImageURL.Detail != "auto" {
		t.Errorf("parts[1] = %+v", parts[1])
	}
}

// TestConvertClaudeAssistantMessage 文本 / thinking / tool_use 块的 assistant 消息转换。
func TestConvertClaudeAssistantMessage(t *testing.T) {
	msg := convertClaudeAssistantMessage(dto.ClaudeMessage{
		Role: "assistant",
		Content: []any{
			map[string]any{"type": "thinking", "thinking": "let me think"},
			map[string]any{"type": "text", "text": "answer"},
			map[string]any{
				"type": "tool_use", "id": "toolu_9", "name": "get_weather",
				"input": map[string]any{"city": "北京"},
			},
		},
	})
	if msg.Role != "assistant" {
		t.Errorf("role = %q", msg.Role)
	}
	if msg.Content != "answer" {
		t.Errorf("content = %v, want answer", msg.Content)
	}
	if msg.ReasoningContent == nil || *msg.ReasoningContent != "let me think" {
		t.Errorf("ReasoningContent = %v, want let me think", msg.ReasoningContent)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(msg.ToolCalls))
	}
	tc := msg.ToolCalls[0]
	if tc.ID != "toolu_9" || tc.Type != "function" || tc.Function.Name != "get_weather" {
		t.Errorf("tool call = %+v", tc)
	}
	if tc.Function.Arguments != `{"city":"北京"}` {
		t.Errorf("arguments = %q", tc.Function.Arguments)
	}
}

// Claude 侧只带服务端内置工具（web_search）时，函数工具列表为空，
// tool_choice 必须一并丢弃——OpenAI 同样拒绝「有 tool_choice 无 tools」的请求体。
func TestClaudeToOpenAIRequestConverter_BuiltinOnlyToolsDropsToolChoice(t *testing.T) {
	converter := &ClaudeToOpenAIRequestConverter{}
	claudeReq := &dto.ClaudeRequest{
		Model:      "claude-sonnet-4",
		Tools:      []dto.ClaudeTool{{Type: "web_search_20250305", Name: "web_search"}},
		ToolChoice: map[string]any{"type": "any"},
		Messages:   []dto.ClaudeMessage{{Role: "user", Content: "搜一下"}},
	}

	result, err := converter.ConvertRequest(context.Background(), newClaudeInboundMeta(), claudeReq)
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	openaiReq := result.(*dto.GeneralOpenAIRequest)

	if len(openaiReq.Tools) != 0 {
		t.Errorf("Tools = %+v, want empty", openaiReq.Tools)
	}
	if openaiReq.ToolChoice != nil {
		t.Errorf("ToolChoice = %v, want nil without tools", openaiReq.ToolChoice)
	}
	// 搜索能力本身不能丢：走 web_search_options 承载
	if openaiReq.WebSearchOptions == nil {
		t.Error("WebSearchOptions is nil, web search capability lost")
	}
}
