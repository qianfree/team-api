package oai_chat

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

func TestOpenAIToClaudeResponseConverter_Metadata(t *testing.T) {
	converter := &OpenAIToClaudeResponseConverter{}

	if converter.ID() != relayconvert.ResponseConverterOAIChatToClaudeMessages {
		t.Errorf("ID() = %q, want %q", converter.ID(), relayconvert.ResponseConverterOAIChatToClaudeMessages)
	}
	if converter.From() != types.RelayFormatOpenAI {
		t.Errorf("From() = %q, want %q", converter.From(), types.RelayFormatOpenAI)
	}
	if converter.To() != types.RelayFormatClaude {
		t.Errorf("To() = %q, want %q", converter.To(), types.RelayFormatClaude)
	}
	if converter.Quality() != relayconvert.ResponseConverterQualityGood {
		t.Errorf("Quality() = %q, want %q", converter.Quality(), relayconvert.ResponseConverterQualityGood)
	}
}

func TestOpenAIToClaudeResponseConverter_InvalidType(t *testing.T) {
	converter := &OpenAIToClaudeResponseConverter{}
	if _, err := converter.ConvertResponse(context.Background(), newClaudeInboundMeta(), "not a response"); err == nil {
		t.Fatal("expected type assertion error, got nil")
	}
}

// TestOpenAIToClaudeResponseConverter_UsageNilDetails 上游不返回 prompt_tokens_details
// （多数 OpenAI 兼容上游）时不得空指针 panic，且无缓存可扣减。
func TestOpenAIToClaudeResponseConverter_UsageNilDetails(t *testing.T) {
	converter := &OpenAIToClaudeResponseConverter{}
	openaiResp := &dto.ChatCompletionResponse{
		ID:      "chatcmpl-1",
		Model:   "gpt-4o",
		Choices: []dto.Choice{{Index: 0, Message: dto.Message{Role: "assistant", Content: "hi"}, FinishReason: "stop"}},
		Usage:   dto.UsageWithDetails{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120},
	}

	result, err := converter.ConvertResponse(context.Background(), newClaudeInboundMeta(), openaiResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	resp := result.(*dto.ClaudeResponse)

	if resp.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100 (no cache to deduct)", resp.Usage.InputTokens)
	}
	if resp.Usage.CacheReadInputTokens != 0 {
		t.Errorf("CacheReadInputTokens = %d, want 0", resp.Usage.CacheReadInputTokens)
	}
	if resp.Usage.OutputTokens != 20 {
		t.Errorf("OutputTokens = %d, want 20", resp.Usage.OutputTokens)
	}
}

// TestOpenAIToClaudeResponseConverter_UsageWithCache 上游返回缓存明细时，usage 需按 Claude 语义映射：
// OpenAI 的 prompt_tokens 含 cached（子集），Claude 的 input_tokens 不含 cache_read，
// 直接透传会导致客户端按本协议语义计账时缓存部分重复计入。
func TestOpenAIToClaudeResponseConverter_UsageWithCache(t *testing.T) {
	converter := &OpenAIToClaudeResponseConverter{}
	openaiResp := &dto.ChatCompletionResponse{
		ID:      "chatcmpl-2",
		Model:   "gpt-4o",
		Choices: []dto.Choice{{Index: 0, Message: dto.Message{Role: "assistant", Content: "hi"}, FinishReason: "stop"}},
		Usage: dto.UsageWithDetails{
			PromptTokens:        100,
			CompletionTokens:    20,
			TotalTokens:         120,
			PromptTokensDetails: &dto.TokenDetails{CachedTokens: 30},
		},
	}

	result, err := converter.ConvertResponse(context.Background(), newClaudeInboundMeta(), openaiResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	resp := result.(*dto.ClaudeResponse)

	if resp.Usage.InputTokens != 70 {
		t.Errorf("InputTokens = %d, want 70 (prompt 100 - cached 30)", resp.Usage.InputTokens)
	}
	if resp.Usage.CacheReadInputTokens != 30 {
		t.Errorf("CacheReadInputTokens = %d, want 30", resp.Usage.CacheReadInputTokens)
	}
}

// TestOpenAIToClaudeResponseConverter_ContentBlocks thinking / text / tool_use 块按序生成。
func TestOpenAIToClaudeResponseConverter_ContentBlocks(t *testing.T) {
	converter := &OpenAIToClaudeResponseConverter{}
	reasoning := "let me think"
	openaiResp := &dto.ChatCompletionResponse{
		ID:    "chatcmpl-3",
		Model: "gpt-4o",
		Choices: []dto.Choice{{
			Index: 0,
			Message: dto.Message{
				Role:             "assistant",
				Content:          "answer",
				ReasoningContent: &reasoning,
				ToolCalls: []dto.ToolCall{{
					ID: "call_1", Type: "function",
					Function: dto.FunctionCall{Name: "get_weather", Arguments: `{"city":"北京"}`},
				}},
			},
			FinishReason: "tool_calls",
		}},
	}

	result, err := converter.ConvertResponse(context.Background(), newClaudeInboundMeta(), openaiResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	resp := result.(*dto.ClaudeResponse)

	if resp.Type != "message" || resp.Role != "assistant" {
		t.Errorf("type/role = %q/%q", resp.Type, resp.Role)
	}
	if !strings.HasPrefix(resp.ID, "msg_") {
		t.Errorf("ID = %q, want msg_ prefix", resp.ID)
	}
	if len(resp.Content) != 3 {
		t.Fatalf("got %d content blocks, want 3 (thinking, text, tool_use)", len(resp.Content))
	}
	if resp.Content[0].Type != "thinking" || resp.Content[0].Thinking == nil || *resp.Content[0].Thinking != "let me think" {
		t.Errorf("block[0] = %+v, want thinking", resp.Content[0])
	}
	if resp.Content[1].Type != "text" || resp.Content[1].Text == nil || *resp.Content[1].Text != "answer" {
		t.Errorf("block[1] = %+v, want text", resp.Content[1])
	}
	if resp.Content[2].Type != "tool_use" || resp.Content[2].ID != "call_1" || resp.Content[2].Name != "get_weather" {
		t.Errorf("block[2] = %+v, want tool_use", resp.Content[2])
	}
	input, ok := resp.Content[2].Input.(map[string]any)
	if !ok || input["city"] != "北京" {
		t.Errorf("tool_use input = %v", resp.Content[2].Input)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("StopReason = %q, want tool_use", resp.StopReason)
	}
}

// TestOpenAIToClaudeResponseConverter_BadToolArguments 工具参数非法 JSON 时兜底为空对象。
func TestOpenAIToClaudeResponseConverter_BadToolArguments(t *testing.T) {
	converter := &OpenAIToClaudeResponseConverter{}
	openaiResp := &dto.ChatCompletionResponse{
		Model: "gpt-4o",
		Choices: []dto.Choice{{
			Message: dto.Message{
				Role: "assistant",
				ToolCalls: []dto.ToolCall{{
					ID: "call_bad", Type: "function",
					Function: dto.FunctionCall{Name: "fn", Arguments: "{invalid"},
				}},
			},
			FinishReason: "tool_calls",
		}},
	}

	result, err := converter.ConvertResponse(context.Background(), newClaudeInboundMeta(), openaiResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	resp := result.(*dto.ClaudeResponse)
	if len(resp.Content) != 1 || resp.Content[0].Type != "tool_use" {
		t.Fatalf("content = %+v", resp.Content)
	}
	input, ok := resp.Content[0].Input.(map[string]any)
	if !ok || len(input) != 0 {
		t.Errorf("input = %v, want empty map", resp.Content[0].Input)
	}
}

// TestOpenAIToClaudeResponseConverter_EmptyContent 空响应兜底为单个空文本块。
func TestOpenAIToClaudeResponseConverter_EmptyContent(t *testing.T) {
	converter := &OpenAIToClaudeResponseConverter{}
	openaiResp := &dto.ChatCompletionResponse{Model: "gpt-4o"}

	result, err := converter.ConvertResponse(context.Background(), newClaudeInboundMeta(), openaiResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	resp := result.(*dto.ClaudeResponse)
	if len(resp.Content) != 1 || resp.Content[0].Type != "text" || resp.Content[0].Text == nil || *resp.Content[0].Text != "" {
		t.Errorf("content = %+v, want single empty text block", resp.Content)
	}
	// 无 choices 时 stop_reason 兜底 end_turn
	if resp.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want end_turn", resp.StopReason)
	}
}

// TestOpenAIToClaudeResponseConverter_ModelName 模型映射时回显入站模型名，
// 未映射时透传上游响应模型名。
func TestOpenAIToClaudeResponseConverter_ModelName(t *testing.T) {
	converter := &OpenAIToClaudeResponseConverter{}
	openaiResp := &dto.ChatCompletionResponse{
		Model:   "gpt-4o-2024-08-06",
		Choices: []dto.Choice{{Message: dto.Message{Role: "assistant", Content: "hi"}, FinishReason: "stop"}},
	}

	// 已映射（origin != upstream）→ 回显 origin
	result, err := converter.ConvertResponse(context.Background(), newClaudeInboundMeta(), openaiResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	if got := result.(*dto.ClaudeResponse).Model; got != "claude-sonnet-4" {
		t.Errorf("mapped model = %q, want claude-sonnet-4", got)
	}

	// 未映射（origin == upstream）→ 透传响应模型名
	unmapped := &convmeta.Values{
		OriginModelName:     "gpt-4o",
		UpstreamModelName:   "gpt-4o",
		ChannelMetaAttached: true,
	}
	result, err = converter.ConvertResponse(context.Background(), unmapped, openaiResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	if got := result.(*dto.ClaudeResponse).Model; got != "gpt-4o-2024-08-06" {
		t.Errorf("unmapped model = %q, want gpt-4o-2024-08-06", got)
	}
}

func TestMapOpenAIFinishReasonToClaude(t *testing.T) {
	tests := map[string]string{
		"stop":           "end_turn",
		"length":         "max_tokens",
		"tool_calls":     "tool_use",
		"content_filter": "refusal",
		"":               "end_turn",
		"custom_reason":  "custom_reason", // 未知值原样透传
	}
	for in, want := range tests {
		if got := mapOpenAIFinishReasonToClaude(in); got != want {
			t.Errorf("mapOpenAIFinishReasonToClaude(%q) = %q, want %q", in, got, want)
		}
	}
}
