package coze_chat

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

// TestConverterMetadata 覆盖响应/流式转换器的 ID/From/To/Quality 元数据方法。
func TestConverterMetadata(t *testing.T) {
	resp := &CozeToOpenAIResponseConverter{}
	if resp.ID() != relayconvert.ResponseConverterCozeChatToOAIChat {
		t.Errorf("resp ID = %q", resp.ID())
	}
	if resp.From() != types.RelayFormatCoze || resp.To() != types.RelayFormatOpenAI {
		t.Errorf("resp From/To = %q/%q", resp.From(), resp.To())
	}
	if resp.Quality() != relayconvert.ResponseConverterQualityFair {
		t.Errorf("resp Quality = %q", resp.Quality())
	}

	stream := &CozeToOpenAIStreamConverter{}
	if stream.ID() != relayconvert.ResponseConverterCozeChatToOAIChatStream {
		t.Errorf("stream ID = %q", stream.ID())
	}
	if stream.From() != types.RelayFormatCoze || stream.To() != types.RelayFormatOpenAI {
		t.Errorf("stream From/To = %q/%q", stream.From(), stream.To())
	}
	if stream.Quality() != relayconvert.ResponseConverterQualityFair {
		t.Errorf("stream Quality = %q", stream.Quality())
	}
}
