package oai_chat

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

func ptrInt(v int) *int { return &v }
func ptrStr(v string) *string { return &v }

func benchOAIRequest() *dto.GeneralOpenAIRequest {
	return &dto.GeneralOpenAIRequest{
		Model:    "gpt-4",
		MaxTokens: ptrInt(512),
		Messages: []dto.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Explain how a hash map works in three sentences."},
		},
	}
}

func benchClaudeMeta() *convmeta.Values {
	return &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "gpt-4",
		UpstreamModelName:   "claude-3-5-sonnet-20241022",
		Options: &convmeta.Options{
			Claude: convmeta.ClaudeOptions{DefaultMaxTokens: func(string) int { return 4096 }},
		},
	}
}

func BenchmarkOpenAIToClaudeRequest(b *testing.B) {
	conv := &OpenAIToClaudeRequestConverter{}
	req := benchOAIRequest()
	info := benchClaudeMeta()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := conv.ConvertRequest(ctx, info, req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClaudeToOpenAIResponse(b *testing.B) {
	conv := &ClaudeToOpenAIResponseConverter{}
	resp := &dto.ClaudeResponse{
		ID: "msg_bench", Type: "message", Role: "assistant", Model: "claude-x", StopReason: "end_turn",
		Content: []dto.ClaudeContentBlock{{Type: "text", Text: ptrStr("A hash map stores key-value pairs in buckets derived from a hash of the key.")}},
		Usage:   &dto.ClaudeUsage{InputTokens: 42, OutputTokens: 18},
	}
	info := benchClaudeMeta()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := conv.ConvertResponse(ctx, info, resp); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClaudeToOpenAIStream(b *testing.B) {
	conv := &ClaudeToOpenAIStreamConverter{}
	info := benchClaudeMeta()
	ctx := context.Background()
	// 一段典型 Claude SSE（message_start + 若干 text_delta + message_delta/stop）
	sse := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-x\",\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\", world\"}}\n\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":7}}\n\n" +
		"data: {\"type\":\"message_stop\"}\n\n"
	writer := func(any) error { return nil }
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := conv.ConvertStreamResponse(ctx, info, strings.NewReader(sse), writer); err != nil {
			b.Fatal(err)
		}
	}
}
