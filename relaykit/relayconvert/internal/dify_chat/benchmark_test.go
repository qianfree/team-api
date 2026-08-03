package dify_chat

import (
	"context"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

func BenchmarkOpenAIToDifyRequest(b *testing.B) {
	conv := &OpenAIToDifyRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4",
		Messages: []dto.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Translate the following text to French."},
		},
	}
	info := &convmeta.Values{ChannelMetaAttached: true, UpstreamModelName: "dify-bot", IsStream: true}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := conv.ConvertRequest(ctx, info, req); err != nil {
			b.Fatal(err)
		}
	}
}
