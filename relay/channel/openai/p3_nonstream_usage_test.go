package openai

// P3 非流式计费回归：claude/gemini 客户端 × ChatViaResponses 渠道（UseResponsesAPI），
// 上游体为 /v1/responses 的 Responses 格式。修复前上方按 chat 格式解析 responses 体必得
// 零值，桥接命中后返回的计费 usage 恒 0（免费请求）；修复后从上游 responses 体提取。

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

// p3ResponsesUpstreamBody 上游 /v1/responses 的非流式响应体（usage.input_tokens=50、output=20）
const p3ResponsesUpstreamBody = `{"object":"response","id":"resp_123","model":"gpt-5","status":"completed","usage":{"input_tokens":50,"output_tokens":20,"total_tokens":70,"input_tokens_details":{"cached_tokens":8}},"output":[{"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"hi"}]}]}`

// newP3NonStreamInfo 构造 claude/gemini 客户端打 ChatViaResponses 渠道的非流式 RelayInfo
func newP3NonStreamInfo(clientFmt constant.RelayFormat, relayMode constant.RelayMode) *common.RelayInfo {
	return &common.RelayInfo{
		InboundFormat:   clientFmt,
		ClientFormat:    clientFmt,
		RelayMode:       int(relayMode),
		UseResponsesAPI: true,
		OriginModelName: "gpt-5",
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			UpstreamModelName: "gpt-5",
		},
	}
}

func p3UpstreamResp() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(p3ResponsesUpstreamBody)),
		Header:     http.Header{},
	}
}

// TestP3NonStreamUsage_ClaudeClient claude 客户端 × responses 上游非流式：
// 响应转 claude 格式写出，且计费 usage 从上游 responses 体提取（非零）。
func TestP3NonStreamUsage_ClaudeClient(t *testing.T) {
	info := newP3NonStreamInfo(constant.RelayFormatClaude, constant.RelayModeClaudeMessages)
	rec := httptest.NewRecorder()

	usage, err := handleClaudeInboundNonStream(context.Background(), p3UpstreamResp(), info, rec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(rec.Body.String(), `{"id":"msg_`) {
		t.Errorf("响应应为 claude 格式，实际: %.80s", rec.Body.String())
	}
	if usage == nil || usage.PromptTokens != 50 || usage.CompletionTokens != 20 {
		t.Errorf("usage = %+v, want prompt=50 completion=20（修复前恒 0）", usage)
	}
	// OpenAI 口径：prompt 已含缓存，cached 为子集
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 8 {
		t.Errorf("cached tokens 未提取: %+v", usage.PromptTokensDetails)
	}
}

// TestP3NonStreamUsage_GeminiClient gemini 客户端 × responses 上游非流式：同上。
func TestP3NonStreamUsage_GeminiClient(t *testing.T) {
	info := newP3NonStreamInfo(constant.RelayFormatGemini, constant.RelayModeGeminiChat)
	rec := httptest.NewRecorder()

	usage, err := handleGeminiInboundNonStream(context.Background(), p3UpstreamResp(), info, rec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(rec.Body.String(), `{"candidates"`) {
		t.Errorf("响应应为 gemini 格式，实际: %.80s", rec.Body.String())
	}
	if usage == nil || usage.PromptTokens != 50 || usage.CompletionTokens != 20 {
		t.Errorf("usage = %+v, want prompt=50 completion=20（修复前恒 0）", usage)
	}
}
