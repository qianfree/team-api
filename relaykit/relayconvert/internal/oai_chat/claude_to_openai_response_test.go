package oai_chat

import (
	"context"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

func TestClaudeToOpenAIResponseConverter_Metadata(t *testing.T) {
	converter := &ClaudeToOpenAIResponseConverter{}

	if converter.ID() != relayconvert.ConverterClaudeMessagesToOpenAIChat {
		t.Errorf("ID() = %q, want %q", converter.ID(), relayconvert.ConverterClaudeMessagesToOpenAIChat)
	}

	if converter.From() != types.RelayFormatClaude {
		t.Errorf("From() = %q, want %q", converter.From(), types.RelayFormatClaude)
	}

	if converter.To() != types.RelayFormatOpenAI {
		t.Errorf("To() = %q, want %q", converter.To(), types.RelayFormatOpenAI)
	}

	if converter.Quality() != relayconvert.ResponseConverterQualityGood {
		t.Errorf("Quality() = %q, want %q", converter.Quality(), relayconvert.ResponseConverterQualityGood)
	}
}

func TestClaudeToOpenAIResponseConverter_BasicConversion(t *testing.T) {
	converter := &ClaudeToOpenAIResponseConverter{}
	ctx := context.Background()

	textContent := "Hello, how can I help you?"
	claudeResp := &dto.ClaudeResponse{
		ID:         "msg_123",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-3-opus-20240229",
		StopReason: "end_turn",
		Content: []dto.ClaudeContentBlock{
			{
				Type: "text",
				Text: &textContent,
			},
		},
		Usage: &dto.ClaudeUsage{
			InputTokens:  10,
			OutputTokens: 20,
		},
	}

	info := &mockMeta{
		originModel: "gpt-4",
	}

	result, err := converter.ConvertResponse(ctx, info, claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp, ok := result.(*dto.ChatCompletionResponse)
	if !ok {
		t.Fatalf("Expected *dto.ChatCompletionResponse, got %T", result)
	}

	if openaiResp.ID != "msg_123" {
		t.Errorf("ID = %q, want %q", openaiResp.ID, "msg_123")
	}

	if openaiResp.Object != "chat.completion" {
		t.Errorf("Object = %q, want %q", openaiResp.Object, "chat.completion")
	}

	if openaiResp.Model != "claude-3-opus-20240229" {
		t.Errorf("Model = %q, want %q", openaiResp.Model, "claude-3-opus-20240229")
	}

	if len(openaiResp.Choices) != 1 {
		t.Fatalf("Expected 1 choice, got %d", len(openaiResp.Choices))
	}

	choice := openaiResp.Choices[0]
	if choice.Index != 0 {
		t.Errorf("Choice index = %d, want 0", choice.Index)
	}

	if choice.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want %q", choice.FinishReason, "stop")
	}

	if choice.Message.Role != "assistant" {
		t.Errorf("Message role = %q, want %q", choice.Message.Role, "assistant")
	}

	if choice.Message.Content != "Hello, how can I help you?" {
		t.Errorf("Message content = %q, want %q", choice.Message.Content, "Hello, how can I help you?")
	}

	if openaiResp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", openaiResp.Usage.PromptTokens)
	}

	if openaiResp.Usage.CompletionTokens != 20 {
		t.Errorf("CompletionTokens = %d, want 20", openaiResp.Usage.CompletionTokens)
	}

	if openaiResp.Usage.TotalTokens != 30 {
		t.Errorf("TotalTokens = %d, want 30", openaiResp.Usage.TotalTokens)
	}
}

func TestClaudeToOpenAIResponseConverter_ThinkingContent(t *testing.T) {
	converter := &ClaudeToOpenAIResponseConverter{}
	ctx := context.Background()

	textContent := "The answer is 42"
	thinkingContent := "Let me think about this carefully..."

	claudeResp := &dto.ClaudeResponse{
		ID:         "msg_456",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-3-opus-20240229",
		StopReason: "end_turn",
		Content: []dto.ClaudeContentBlock{
			{
				Type:     "thinking",
				Thinking: &thinkingContent,
			},
			{
				Type: "text",
				Text: &textContent,
			},
		},
		Usage: &dto.ClaudeUsage{
			InputTokens:  15,
			OutputTokens: 25,
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp := result.(*dto.ChatCompletionResponse)

	message := openaiResp.Choices[0].Message

	if message.Content != "The answer is 42" {
		t.Errorf("Content = %q, want %q", message.Content, "The answer is 42")
	}

	if message.ReasoningContent == nil {
		t.Fatal("Expected ReasoningContent to be set, got nil")
	}

	if *message.ReasoningContent != "Let me think about this carefully..." {
		t.Errorf("ReasoningContent = %q, want %q", *message.ReasoningContent, "Let me think about this carefully...")
	}
}

func TestClaudeToOpenAIResponseConverter_ToolCalls(t *testing.T) {
	converter := &ClaudeToOpenAIResponseConverter{}
	ctx := context.Background()

	claudeResp := &dto.ClaudeResponse{
		ID:         "msg_789",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-3-opus-20240229",
		StopReason: "tool_use",
		Content: []dto.ClaudeContentBlock{
			{
				Type: "tool_use",
				ID:   "call_abc",
				Name: "get_weather",
				Input: map[string]any{
					"city": "San Francisco",
				},
			},
		},
		Usage: &dto.ClaudeUsage{
			InputTokens:  20,
			OutputTokens: 10,
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp := result.(*dto.ChatCompletionResponse)

	choice := openaiResp.Choices[0]

	if choice.FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want %q", choice.FinishReason, "tool_calls")
	}

	message := choice.Message

	if len(message.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(message.ToolCalls))
	}

	toolCall := message.ToolCalls[0]

	if toolCall.ID != "call_abc" {
		t.Errorf("ToolCall ID = %q, want %q", toolCall.ID, "call_abc")
	}

	if toolCall.Type != "function" {
		t.Errorf("ToolCall Type = %q, want %q", toolCall.Type, "function")
	}

	if toolCall.Function.Name != "get_weather" {
		t.Errorf("Function name = %q, want %q", toolCall.Function.Name, "get_weather")
	}

	if toolCall.Function.Arguments != `{"city":"San Francisco"}` {
		t.Errorf("Function arguments = %q, want %q", toolCall.Function.Arguments, `{"city":"San Francisco"}`)
	}
}

func TestClaudeToOpenAIResponseConverter_MultipleTextBlocks(t *testing.T) {
	converter := &ClaudeToOpenAIResponseConverter{}
	ctx := context.Background()

	text1 := "First paragraph."
	text2 := "Second paragraph."

	claudeResp := &dto.ClaudeResponse{
		ID:         "msg_multi",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-3-opus-20240229",
		StopReason: "end_turn",
		Content: []dto.ClaudeContentBlock{
			{
				Type: "text",
				Text: &text1,
			},
			{
				Type: "text",
				Text: &text2,
			},
		},
		Usage: &dto.ClaudeUsage{
			InputTokens:  5,
			OutputTokens: 10,
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp := result.(*dto.ChatCompletionResponse)
	message := openaiResp.Choices[0].Message

	expectedContent := "First paragraph.\nSecond paragraph."
	if message.Content != expectedContent {
		t.Errorf("Content = %q, want %q", message.Content, expectedContent)
	}
}

func TestClaudeToOpenAIResponseConverter_StopReasonMapping(t *testing.T) {
	tests := []struct {
		name              string
		claudeStopReason  string
		expectedFinish    string
	}{
		{
			name:             "end_turn maps to stop",
			claudeStopReason: "end_turn",
			expectedFinish:   "stop",
		},
		{
			name:             "max_tokens maps to length",
			claudeStopReason: "max_tokens",
			expectedFinish:   "length",
		},
		{
			name:             "tool_use maps to tool_calls",
			claudeStopReason: "tool_use",
			expectedFinish:   "tool_calls",
		},
		{
			name:             "stop_sequence maps to stop",
			claudeStopReason: "stop_sequence",
			expectedFinish:   "stop",
		},
		{
			name:             "unknown maps to stop",
			claudeStopReason: "unknown_reason",
			expectedFinish:   "stop",
		},
	}

	converter := &ClaudeToOpenAIResponseConverter{}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			textContent := "test"
			claudeResp := &dto.ClaudeResponse{
				ID:         "msg_test",
				Type:       "message",
				Role:       "assistant",
				Model:      "claude-3-opus-20240229",
				StopReason: tt.claudeStopReason,
				Content: []dto.ClaudeContentBlock{
					{
						Type: "text",
						Text: &textContent,
					},
				},
				Usage: &dto.ClaudeUsage{
					InputTokens:  1,
					OutputTokens: 1,
				},
			}

			result, err := converter.ConvertResponse(ctx, nil, claudeResp)
			if err != nil {
				t.Fatalf("ConvertResponse failed: %v", err)
			}

			openaiResp := result.(*dto.ChatCompletionResponse)

			if openaiResp.Choices[0].FinishReason != tt.expectedFinish {
				t.Errorf("FinishReason = %q, want %q", openaiResp.Choices[0].FinishReason, tt.expectedFinish)
			}
		})
	}
}

func TestClaudeToOpenAIResponseConverter_EmptyContent(t *testing.T) {
	converter := &ClaudeToOpenAIResponseConverter{}
	ctx := context.Background()

	claudeResp := &dto.ClaudeResponse{
		ID:         "msg_empty",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-3-opus-20240229",
		StopReason: "end_turn",
		Content:    []dto.ClaudeContentBlock{},
		Usage: &dto.ClaudeUsage{
			InputTokens:  5,
			OutputTokens: 0,
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp := result.(*dto.ChatCompletionResponse)
	message := openaiResp.Choices[0].Message

	if message.Role != "assistant" {
		t.Errorf("Role = %q, want %q", message.Role, "assistant")
	}

	// Empty content should result in empty string or nil
	if message.Content != "" && message.Content != nil {
		t.Errorf("Expected empty content, got %v", message.Content)
	}
}

func TestClaudeToOpenAIResponseConverter_ModelNameFallback(t *testing.T) {
	converter := &ClaudeToOpenAIResponseConverter{}
	ctx := context.Background()

	textContent := "test"
	claudeResp := &dto.ClaudeResponse{
		ID:         "msg_test",
		Type:       "message",
		Role:       "assistant",
		Model:      "", // Empty model name
		StopReason: "end_turn",
		Content: []dto.ClaudeContentBlock{
			{
				Type: "text",
				Text: &textContent,
			},
		},
		Usage: &dto.ClaudeUsage{
			InputTokens:  1,
			OutputTokens: 1,
		},
	}

	info := &mockMeta{
		originModel: "gpt-4",
	}

	result, err := converter.ConvertResponse(ctx, info, claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp := result.(*dto.ChatCompletionResponse)

	if openaiResp.Model != "gpt-4" {
		t.Errorf("Model = %q, want %q (fallback from info)", openaiResp.Model, "gpt-4")
	}
}
