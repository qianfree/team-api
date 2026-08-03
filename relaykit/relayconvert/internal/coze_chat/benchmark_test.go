package coze_chat

import (
	"context"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

func BenchmarkOpenAIToCozeRequest(b *testing.B) {
	conv := &OpenAIToCozeRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Model: "bot-origin",
		Messages: []dto.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Summarize the latest question in one line."},
		},
	}
	info := &convmeta.Values{ChannelMetaAttached: true, UpstreamModelName: "bot-mapped"}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := conv.ConvertRequest(ctx, info, req); err != nil {
			b.Fatal(err)
		}
	}
}
