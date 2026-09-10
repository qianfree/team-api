package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// newClaudeInboundTestInfo 构造 Claude 入站转换测试用的 RelayInfo。
func newClaudeInboundTestInfo() *common.RelayInfo {
	return &common.RelayInfo{
		RequestID:       "req-claude-inbound",
		OriginModelName: "gpt-4o",
		StreamStatus:    common.NewStreamStatus(),
		// 客户端说 Claude、上游说 OpenAI：命中 relaykit 的 openai→claude 流式转换器
		InboundFormat: constant.RelayFormatClaude,
		ChannelMeta: &common.ChannelMeta{
			ChannelType: int(constant.ProviderOpenAI),
		},
	}
}

func TestHandleClaudeInboundStream_CacheUsage(t *testing.T) {
	sse := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[],"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120,"prompt_tokens_details":{"cached_tokens":30},"completion_tokens_details":{"reasoning_tokens":5}}}

data: [DONE]

`
	upstream := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(sse)),
		Header:     http.Header{},
	}

	info := newClaudeInboundTestInfo()
	rec := httptest.NewRecorder()

	usage, err := handleClaudeInboundStream(context.Background(), upstream, info, rec)
	if err != nil {
		t.Fatalf("handleClaudeInboundStream error: %v", err)
	}

	// 计费侧：OpenAI 原始口径（prompt 含 cached）+ 明细 + 扣减标记
	if usage.PromptTokens != 100 || usage.CompletionTokens != 20 || usage.TotalTokens != 120 {
		t.Errorf("billing usage = %+v, want prompt=100 completion=20 total=120", usage)
	}
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 30 {
		t.Errorf("cache details = %+v, want cached=30", usage.PromptTokensDetails)
	}
	if !usage.CacheIncludedInPrompt {
		t.Error("CacheIncludedInPrompt = false, want true (OpenAI prompt 含 cached)")
	}

	// 客户端侧：message_delta 补报 Claude 语义 usage
	body := rec.Body.String()
	if !strings.Contains(body, `"input_tokens":70`) {
		t.Errorf("message_delta missing deducted input_tokens=70, got: %s", body)
	}
	if !strings.Contains(body, `"cache_read_input_tokens":30`) {
		t.Errorf("message_delta missing cache_read_input_tokens=30, got: %s", body)
	}
	if !strings.Contains(body, `"output_tokens":20`) {
		t.Errorf("message_delta missing output_tokens=20, got: %s", body)
	}
}
