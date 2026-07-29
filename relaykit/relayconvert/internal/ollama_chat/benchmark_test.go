package ollama_chat

import (
	"context"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

func BenchmarkOpenAIToOllamaRequest(b *testing.B) {
	conv := &OpenAIToOllamaRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Model:    "llama3",
		MaxTokens: func() *int { v := 256; return &v }(),
		Messages: []dto.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Write a haiku about the ocean."},
		},
	}
	info := &convmeta.Values{ChannelMetaAttached: true, UpstreamModelName: "llama3"}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := conv.ConvertRequest(ctx, info, req); err != nil {
			b.Fatal(err)
		}
	}
}
