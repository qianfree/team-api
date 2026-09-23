package oai_gemini

import (
	"context"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

func TestOpenAIToGeminiResponseConverter_Metadata(t *testing.T) {
	converter := &OpenAIToGeminiResponseConverter{}

	if converter.ID() != relayconvert.ResponseConverterOAIChatToGeminiChat {
		t.Errorf("ID() = %q, want %q", converter.ID(), relayconvert.ResponseConverterOAIChatToGeminiChat)
	}

	if converter.From() != types.RelayFormatOpenAI {
		t.Errorf("From() = %q, want %q", converter.From(), types.RelayFormatOpenAI)
	}

	if converter.To() != types.RelayFormatGemini {
		t.Errorf("To() = %q, want %q", converter.To(), types.RelayFormatGemini)
	}

	if converter.Quality() != relayconvert.ResponseConverterQualityGood {
		t.Errorf("Quality() = %q, want %q", converter.Quality(), relayconvert.ResponseConverterQualityGood)
	}
}

func TestOpenAIToGeminiResponseConverter_BasicConversion(t *testing.T) {
	converter := &OpenAIToGeminiResponseConverter{}
	ctx := context.Background()

	openaiResp := &dto.ChatCompletionResponse{
		ID:    "chatcmpl-123",
		Model: "gpt-4o",
		Choices: []dto.Choice{
			{
				Index:        0,
				FinishReason: "stop",
				Message: dto.Message{
					Role:    "assistant",
					Content: "Hello there!",
				},
			},
		},
		Usage: dto.UsageWithDetails{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, openaiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	geminiResp, ok := result.(*dto.GeminiChatResponse)
	if !ok {
		t.Fatalf("Expected *dto.GeminiChatResponse, got %T", result)
	}

	if len(geminiResp.Candidates) != 1 {
		t.Fatalf("Candidates count = %d, want 1", len(geminiResp.Candidates))
	}
	candidate := geminiResp.Candidates[0]
	if candidate.FinishReason != "STOP" {
		t.Errorf("FinishReason = %q, want STOP", candidate.FinishReason)
	}
	if candidate.Content == nil || candidate.Content.Role != "model" {
		t.Fatalf("Candidate content = %+v", candidate.Content)
	}
	if len(candidate.Content.Parts) != 1 || candidate.Content.Parts[0].Text != "Hello there!" {
		t.Errorf("Parts = %+v", candidate.Content.Parts)
	}

	if geminiResp.UsageMetadata == nil {
		t.Fatal("UsageMetadata missing")
	}
	if geminiResp.UsageMetadata.PromptTokenCount != 10 {
		t.Errorf("PromptTokenCount = %d, want 10", geminiResp.UsageMetadata.PromptTokenCount)
	}
	if geminiResp.UsageMetadata.CandidatesTokenCount != 20 {
		t.Errorf("CandidatesTokenCount = %d, want 20", geminiResp.UsageMetadata.CandidatesTokenCount)
	}
	if geminiResp.UsageMetadata.TotalTokenCount != 30 {
		t.Errorf("TotalTokenCount = %d, want 30", geminiResp.UsageMetadata.TotalTokenCount)
	}
}

func TestOpenAIToGeminiResponseConverter_ReasoningContent(t *testing.T) {
	converter := &OpenAIToGeminiResponseConverter{}
	ctx := context.Background()

	reasoning := "Let me think..."
	openaiResp := &dto.ChatCompletionResponse{
		Choices: []dto.Choice{
			{
				FinishReason: "stop",
				Message: dto.Message{
					Role:             "assistant",
					Content:          "The answer is 42.",
					ReasoningContent: &reasoning,
				},
			},
		},
		Usage: dto.UsageWithDetails{
			PromptTokens:           10,
			CompletionTokens:       50,
			TotalTokens:            60,
			CompletionTokenDetails: &dto.TokenDetails{ReasoningTokens: 30},
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, openaiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}
	geminiResp := result.(*dto.GeminiChatResponse)

	parts := geminiResp.Candidates[0].Content.Parts
	if len(parts) != 2 {
		t.Fatalf("Parts count = %d, want 2", len(parts))
	}
	// thinking 内容排在前面且带 thought 标记
	if parts[0].Text != "Let me think..." || parts[0].Thought == nil || !*parts[0].Thought {
		t.Errorf("Thought part = %+v", parts[0])
	}
	if parts[1].Text != "The answer is 42." || parts[1].Thought != nil {
		t.Errorf("Text part = %+v", parts[1])
	}

	// Gemini 口径：candidatesTokenCount 不含 reasoning（50 - 30 = 20）
	if geminiResp.UsageMetadata.CandidatesTokenCount != 20 {
		t.Errorf("CandidatesTokenCount = %d, want 20", geminiResp.UsageMetadata.CandidatesTokenCount)
	}
	if geminiResp.UsageMetadata.ThoughtsTokenCount != 30 {
		t.Errorf("ThoughtsTokenCount = %d, want 30", geminiResp.UsageMetadata.ThoughtsTokenCount)
	}
	if geminiResp.UsageMetadata.TotalTokenCount != 60 {
		t.Errorf("TotalTokenCount = %d, want 60", geminiResp.UsageMetadata.TotalTokenCount)
	}
}

func TestOpenAIToGeminiResponseConverter_ToolCalls(t *testing.T) {
	converter := &OpenAIToGeminiResponseConverter{}
	ctx := context.Background()

	openaiResp := &dto.ChatCompletionResponse{
		Choices: []dto.Choice{
			{
				FinishReason: "tool_calls",
				Message: dto.Message{
					Role: "assistant",
					ToolCalls: []dto.ToolCall{
						{
							ID:   "call_abc",
							Type: "function",
							Function: dto.FunctionCall{
								Name:      "get_weather",
								Arguments: `{"city":"北京"}`,
							},
						},
						{
							ID:   "call_def",
							Type: "function",
							Function: dto.FunctionCall{
								Name:      "get_time",
								Arguments: `invalid json`,
							},
						},
					},
				},
			},
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, openaiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}
	geminiResp := result.(*dto.GeminiChatResponse)

	candidate := geminiResp.Candidates[0]
	// tool_calls → STOP（Gemini 无独立工具调用结束原因）
	if candidate.FinishReason != "STOP" {
		t.Errorf("FinishReason = %q, want STOP", candidate.FinishReason)
	}

	parts := candidate.Content.Parts
	if len(parts) != 2 {
		t.Fatalf("Parts count = %d, want 2", len(parts))
	}
	fc := parts[0].FunctionCall
	if fc == nil || fc.ID != "call_abc" || fc.FunctionName != "get_weather" {
		t.Fatalf("FunctionCall part = %+v", parts[0])
	}
	args, ok := fc.Arguments.(map[string]any)
	if !ok || args["city"] != "北京" {
		t.Errorf("Arguments = %v", fc.Arguments)
	}
	// 非法 JSON 参数降级为空 map
	fc2 := parts[1].FunctionCall
	if fc2 == nil {
		t.Fatal("Second FunctionCall missing")
	}
	args2, ok := fc2.Arguments.(map[string]any)
	if !ok || len(args2) != 0 {
		t.Errorf("Invalid-JSON arguments = %v, want empty map", fc2.Arguments)
	}
}

func TestOpenAIToGeminiResponseConverter_EmptyChoices(t *testing.T) {
	converter := &OpenAIToGeminiResponseConverter{}
	ctx := context.Background()

	result, err := converter.ConvertResponse(ctx, nil, &dto.ChatCompletionResponse{})
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}
	geminiResp := result.(*dto.GeminiChatResponse)
	if len(geminiResp.Candidates) != 0 {
		t.Errorf("Candidates = %+v, want empty", geminiResp.Candidates)
	}
	// 空 choices 时不写 usage
	if geminiResp.UsageMetadata != nil {
		t.Errorf("UsageMetadata = %+v, want nil", geminiResp.UsageMetadata)
	}
}

func TestOpenAIToGeminiResponseConverter_EmptyMessageFallbackPart(t *testing.T) {
	converter := &OpenAIToGeminiResponseConverter{}
	ctx := context.Background()

	openaiResp := &dto.ChatCompletionResponse{
		Choices: []dto.Choice{
			{FinishReason: "stop", Message: dto.Message{Role: "assistant"}},
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, openaiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}
	geminiResp := result.(*dto.GeminiChatResponse)

	// 空消息兜底一个空 text part
	parts := geminiResp.Candidates[0].Content.Parts
	if len(parts) != 1 || parts[0].Text != "" || parts[0].FunctionCall != nil {
		t.Errorf("Parts = %+v, want single empty text part", parts)
	}
}

func TestOpenAIToGeminiResponseConverter_CachedTokens(t *testing.T) {
	converter := &OpenAIToGeminiResponseConverter{}
	ctx := context.Background()

	openaiResp := &dto.ChatCompletionResponse{
		Choices: []dto.Choice{
			{FinishReason: "stop", Message: dto.Message{Role: "assistant", Content: "hi"}},
		},
		Usage: dto.UsageWithDetails{
			PromptTokens:        100,
			CompletionTokens:    10,
			TotalTokens:         110,
			PromptTokensDetails: &dto.TokenDetails{CachedTokens: 80},
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, openaiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}
	geminiResp := result.(*dto.GeminiChatResponse)

	if geminiResp.UsageMetadata.CachedContentTokenCount != 80 {
		t.Errorf("CachedContentTokenCount = %d, want 80", geminiResp.UsageMetadata.CachedContentTokenCount)
	}
}

func TestMapOpenAIFinishReasonToGemini(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"stop", "STOP"},
		{"length", "MAX_TOKENS"},
		{"content_filter", "SAFETY"},
		{"tool_calls", "STOP"},
		{"", ""},
		{"custom_reason", "custom_reason"},
	}

	for _, tt := range tests {
		if got := mapOpenAIFinishReasonToGemini(tt.in); got != tt.want {
			t.Errorf("mapOpenAIFinishReasonToGemini(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestOpenAIToGeminiResponseConverter_InvalidResponseType(t *testing.T) {
	converter := &OpenAIToGeminiResponseConverter{}
	ctx := context.Background()

	if _, err := converter.ConvertResponse(ctx, nil, "not a response"); err == nil {
		t.Error("Expected error for invalid response type, got nil")
	}
}
