package oai_gemini

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

func benchGeminiMeta() *convmeta.Values {
	return &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "gpt-4",
		UpstreamModelName:   "gemini-2.0-flash",
	}
}

func BenchmarkOpenAIToGeminiRequest(b *testing.B) {
	conv := &OpenAIToGeminiRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4",
		Messages: []dto.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Explain how a hash map works in three sentences."},
		},
	}
	info := benchGeminiMeta()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := conv.ConvertRequest(ctx, info, req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGeminiToOpenAIResponse(b *testing.B) {
	conv := &GeminiToOpenAIResponseConverter{}
	resp := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{{
			Content:      &dto.GeminiContent{Role: "model", Parts: []dto.GeminiPart{{Text: "A hash map stores key-value pairs in buckets derived from a hash of the key."}}},
			FinishReason: "STOP",
		}},
		UsageMetadata: &dto.GeminiUsageMetadata{PromptTokenCount: 42, CandidatesTokenCount: 18, TotalTokenCount: 60},
	}
	info := benchGeminiMeta()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := conv.ConvertResponse(ctx, info, resp); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGeminiToOpenAIStream(b *testing.B) {
	conv := &GeminiToOpenAIStreamConverter{}
	info := benchGeminiMeta()
	ctx := context.Background()
	sse := "data: {\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Hello!\"}]}}]}\n\n" +
		"data: {\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\" How are you?\"}]}}]}\n\n" +
		"data: {\"candidates\":[{\"index\":0,\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":8,\"candidatesTokenCount\":12,\"totalTokenCount\":20}}\n\n"
	writer := func(any) error { return nil }
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := conv.ConvertStreamResponse(ctx, info, strings.NewReader(sse), writer); err != nil {
			b.Fatal(err)
		}
	}
}
