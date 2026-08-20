package claude

// #18 交叉客户端非流式计费回归：gemini 客户端 × claude 渠道。修复前桥接命中路径
// return nil, nil——上游成功响应却返回 nil usage，结算层 usage.CompletionTokens 直接
// nil 解引用 panic；修复后从上游原生 Claude 体提取（Claude 口径）。

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

const crossClientClaudeUpstreamBody = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":12,"output_tokens":7,"cache_read_input_tokens":4,"cache_creation_input_tokens":2}}`

// TestCrossClientNonStreamUsage_GeminiClientOnClaude gemini 客户端 × claude 上游非流式：
// 响应转 gemini 格式写出，且计费 usage 非 nil、从上游 Claude 体提取。
func TestCrossClientNonStreamUsage_GeminiClientOnClaude(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat:   constant.RelayFormatGemini,
		ClientFormat:    constant.RelayFormatGemini,
		RelayMode:       int(constant.RelayModeGeminiChat),
		OriginModelName: "claude-sonnet-4",
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderClaude),
			UpstreamModelName: "claude-sonnet-4",
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(crossClientClaudeUpstreamBody)),
		Header:     http.Header{},
	}
	rec := httptest.NewRecorder()

	a := &Adaptor{}
	usage, err := a.handleGeminiClientOnClaude(context.Background(), resp, info, rec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage == nil {
		t.Fatal("usage = nil（修复前 return nil, nil 会导致结算层 nil 解引用 panic）")
	}
	// Claude 口径：input 不含缓存（TotalInputTokens 会补加 cache 读/写）
	if usage.PromptTokens != 12 || usage.CompletionTokens != 7 {
		t.Errorf("usage = %+v, want prompt=12 completion=7", usage)
	}
	if usage.PromptTokensDetails == nil ||
		usage.PromptTokensDetails.CachedTokens != 4 || usage.CacheCreationTokens != 2 {
		t.Errorf("cache 明细未提取: %+v", usage.PromptTokensDetails)
	}
	// 响应体为 gemini 格式
	if !strings.HasPrefix(rec.Body.String(), `{"candidates"`) {
		t.Errorf("响应应为 gemini 格式，实际: %.80s", rec.Body.String())
	}
}
