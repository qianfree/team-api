package claude

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

func TestConvertOpenAIToClaude_SimpleChat(t *testing.T) {
	info := &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{
			UpstreamModelName: "claude-3-5-sonnet-20241022",
		},
		InboundFormat: "openai",
	}

	openaiReq := dto.GeneralOpenAIRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []dto.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
	}

	body, _ := json.Marshal(openaiReq)
	reader, err := ConvertOpenAIToClaude(body, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var claudeReq dto.ClaudeRequest
	data, _ := io.ReadAll(reader)
	if err := json.Unmarshal(data, &claudeReq); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if claudeReq.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("expected model claude-3-5-sonnet-20241022, got %s", claudeReq.Model)
	}
	if claudeReq.System != "You are helpful." {
		t.Errorf("expected system 'You are helpful.', got %v", claudeReq.System)
	}
	if len(claudeReq.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(claudeReq.Messages))
	}
	if claudeReq.Messages[0].Role != "user" {
		t.Errorf("expected first message role 'user', got %s", claudeReq.Messages[0].Role)
	}
	if claudeReq.Messages[1].Role != "assistant" {
		t.Errorf("expected second message role 'assistant', got %s", claudeReq.Messages[1].Role)
	}
	if claudeReq.Messages[2].Role != "user" {
		t.Errorf("expected third message role 'user', got %s", claudeReq.Messages[2].Role)
	}
}

func TestConvertOpenAIToClaude_ToolUse(t *testing.T) {
	info := &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{
			UpstreamModelName: "claude-3-5-sonnet-20241022",
		},
		InboundFormat: "openai",
	}

	openaiReq := dto.GeneralOpenAIRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []dto.Message{
			{Role: "user", Content: "What's the weather?"},
			{
				Role: "assistant",
				ToolCalls: []dto.ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Function: dto.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"NYC"}`,
						},
					},
				},
			},
			{Role: "tool", ToolCallID: "call_123", Content: "Sunny, 72°F"},
		},
		Tools: []dto.Tool{
			{
				Type: "function",
				Function: dto.FunctionDef{
					Name:        "get_weather",
					Description: "Get weather info",
					Parameters:  map[string]any{"type": "object"},
				},
			},
		},
	}

	body, _ := json.Marshal(openaiReq)
	reader, err := ConvertOpenAIToClaude(body, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var claudeReq dto.ClaudeRequest
	data, _ := io.ReadAll(reader)
	if err := json.Unmarshal(data, &claudeReq); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(claudeReq.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(claudeReq.Messages))
	}
	assistantMsg := claudeReq.Messages[1]
	if assistantMsg.Role != "assistant" {
		t.Errorf("expected role 'assistant', got %s", assistantMsg.Role)
	}
	toolMsg := claudeReq.Messages[2]
	if toolMsg.Role != "user" {
		t.Errorf("expected role 'user' for tool result, got %s", toolMsg.Role)
	}
	if len(claudeReq.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(claudeReq.Tools))
	}
	if claudeReq.Tools[0].Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got %s", claudeReq.Tools[0].Name)
	}
}

func TestClaudeStopReasonToOpenAI(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"end_turn", "stop"},
		{"stop_sequence", "stop"},
		{"max_tokens", "length"},
		{"tool_use", "tool_calls"},
		{"pause_turn", "stop"},
		{"refusal", "content_filter"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		result := common.ClaudeStopReasonToOpenAI(tt.input)
		if result != tt.expected {
			t.Errorf("ClaudeStopReasonToOpenAI(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestConvertOpenAIToClaude_WithMaxTokens(t *testing.T) {
	info := &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{
			UpstreamModelName: "claude-3-5-sonnet-20241022",
		},
		InboundFormat: "openai",
	}

	maxTokens := 2048
	openaiReq := dto.GeneralOpenAIRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: &maxTokens,
		Messages: []dto.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(openaiReq)
	reader, err := ConvertOpenAIToClaude(body, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var claudeReq dto.ClaudeRequest
	data, _ := io.ReadAll(reader)
	if err := json.Unmarshal(data, &claudeReq); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if claudeReq.MaxTokens == nil {
		t.Fatal("expected MaxTokens to be set")
	}
	if *claudeReq.MaxTokens != 2048 {
		t.Errorf("expected MaxTokens 2048, got %d", *claudeReq.MaxTokens)
	}
}
