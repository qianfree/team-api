package ollama_chat

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

// TestConverterMetadata 覆盖请求/响应/流式转换器的 ID/From/To/Quality 元数据方法。
func TestConverterMetadata(t *testing.T) {
	req := &OpenAIToOllamaRequestConverter{}
	if req.ID() != relayconvert.ConverterOpenAIChatToOllama {
		t.Errorf("req ID = %q", req.ID())
	}
	if req.From() != types.RelayFormatOpenAI || req.To() != types.RelayFormatOllama {
		t.Errorf("req From/To = %q/%q", req.From(), req.To())
	}
	if req.Quality() != relayconvert.RequestConverterQualityGood {
		t.Errorf("req Quality = %q", req.Quality())
	}

	resp := &OllamaToOpenAIResponseConverter{}
	if resp.ID() != relayconvert.ResponseConverterOllamaChatToOAIChat {
		t.Errorf("resp ID = %q", resp.ID())
	}
	if resp.From() != types.RelayFormatOllama || resp.To() != types.RelayFormatOpenAI {
		t.Errorf("resp From/To = %q/%q", resp.From(), resp.To())
	}
	if resp.Quality() != relayconvert.ResponseConverterQualityGood {
		t.Errorf("resp Quality = %q", resp.Quality())
	}

	stream := &OllamaToOpenAIStreamConverter{}
	if stream.ID() != relayconvert.ResponseConverterOllamaChatToOAIChatStream {
		t.Errorf("stream ID = %q", stream.ID())
	}
	if stream.From() != types.RelayFormatOllama || stream.To() != types.RelayFormatOpenAI {
		t.Errorf("stream From/To = %q/%q", stream.From(), stream.To())
	}
	if stream.Quality() != relayconvert.ResponseConverterQualityGood {
		t.Errorf("stream Quality = %q", stream.Quality())
	}
}
