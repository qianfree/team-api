package gemini

// #18 交叉客户端非流式计费回归：claude/responses 客户端 × gemini 渠道。修复前桥接命中
// 路径 return nil, nil——上游成功响应却返回 nil usage，结算层 usage.CompletionTokens
// 直接 nil 解引用 panic；修复后从上游原生 Gemini 体提取（Gemini 口径）。

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"

	// blank import 触发内置转换器注册（与生产桥接路径一致；本包其余测试未引入，
	// 无此导入时注册表为空、桥接恒回退 legacy）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

const crossClientGeminiUpstreamBody = `{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"thoughtsTokenCount":2,"totalTokenCount":17,"cachedContentTokenCount":3},"modelVersion":"gemini-2.0-flash"}`

func newCrossClientInfo(clientFmt constant.RelayFormat, relayMode constant.RelayMode) *common.RelayInfo {
	return &common.RelayInfo{
		InboundFormat:   clientFmt,
		ClientFormat:    clientFmt,
		RelayMode:       int(relayMode),
		OriginModelName: "gemini-2.0-flash",
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderGemini),
			UpstreamModelName: "gemini-2.0-flash",
		},
	}
}

func crossClientUpstreamResp() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(crossClientGeminiUpstreamBody)),
		Header:     http.Header{},
	}
}

// assertGeminiUpstreamUsage 校验 gemini 上游口径：completion = candidates(5) + thoughts(2)，
// prompt 含 cached 子集（CacheIncludedInPrompt=true）
func assertGeminiUpstreamUsage(t *testing.T, usage *common.Usage) {
	t.Helper()
	if usage == nil {
		t.Fatal("usage = nil（修复前 return nil, nil 会导致结算层 nil 解引用 panic）")
	}
	if usage.PromptTokens != 10 || usage.CompletionTokens != 7 {
		t.Errorf("usage = %+v, want prompt=10 completion=7（candidates+thoughts）", usage)
	}
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 3 {
		t.Errorf("cached tokens 未提取: %+v", usage.PromptTokensDetails)
	}
}

// TestCrossClientNonStreamUsage_ClaudeClientOnGemini claude 客户端 × gemini 上游非流式。
func TestCrossClientNonStreamUsage_ClaudeClientOnGemini(t *testing.T) {
	info := newCrossClientInfo(constant.RelayFormatClaude, constant.RelayModeClaudeMessages)
	rec := httptest.NewRecorder()

	a := &Adaptor{}
	usage, err := a.handleCrossClientOnGemini(context.Background(), crossClientUpstreamResp(), info, rec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertGeminiUpstreamUsage(t, usage)
	if !strings.HasPrefix(rec.Body.String(), `{"id":"msg_`) {
		t.Errorf("响应应为 claude 格式，实际: %.80s", rec.Body.String())
	}
}

// TestCrossClientNonStreamUsage_ResponsesClientOnGemini responses 客户端 × gemini 上游非流式。
func TestCrossClientNonStreamUsage_ResponsesClientOnGemini(t *testing.T) {
	info := newCrossClientInfo(constant.RelayFormatResponses, constant.RelayModeResponses)
	rec := httptest.NewRecorder()

	a := &Adaptor{}
	usage, err := a.handleCrossClientOnGemini(context.Background(), crossClientUpstreamResp(), info, rec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertGeminiUpstreamUsage(t, usage)
	if !strings.Contains(rec.Body.String(), `"object":"response"`) {
		t.Errorf("响应应为 responses 格式，实际: %.80s", rec.Body.String())
	}
}
