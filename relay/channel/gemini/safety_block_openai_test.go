package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// openaiInboundAdaptor 构造 OpenAI 入站 + Gemini 上游的已 Init 适配器。
// 三个 Gemini 出站方向（→Claude / →Responses / →OpenAI）共用同一套安全拦截口径，
// 前两者已在各自 bridge_test 覆盖，本文件补齐 →OpenAI 这一路。
func openaiInboundAdaptor(stream bool) (*Adaptor, *common.RelayInfo) {
	info := &common.RelayInfo{
		RelayMode:       int(constant.RelayModeChatCompletions),
		IsStream:        stream,
		RequestID:       "req123",
		InboundFormat:   constant.RelayFormatOpenAI,
		ClientFormat:    constant.RelayFormatOpenAI,
		OriginModelName: "gemini-3-pro",
		StartTime:       time.Now(),
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderGemini),
			BaseURL:           "https://upstream.example.com",
			UpstreamModelName: "gemini-3-pro",
		},
	}
	a := &Adaptor{}
	a.Init(info)
	return a, info
}

// TestHandleNonStreamToOpenAI_SafetyBlock 非流式安全拦截：返回请求类错误（4xx）且不写响应体。
func TestHandleNonStreamToOpenAI_SafetyBlock(t *testing.T) {
	a, info := openaiInboundAdaptor(false)
	rec := httptest.NewRecorder()

	_, err := a.handleNonStreamToOpenAI(context.Background(),
		jsonResponse(http.StatusOK, `{"promptFeedback":{"blockReason":"SAFETY"}}`), info, rec)
	if err == nil {
		t.Fatal("安全过滤应返回错误")
	}
	assertContentBlockedNotUpstream(t, err)
	if rec.Body.Len() != 0 {
		t.Errorf("不应写响应体, got %q", rec.Body.String())
	}
}

// TestHandleStreamToOpenAI_SafetyBlock 流式安全拦截：SSE 头已提交，必须写 [DONE] 收尾
// 并返回带 ResponseWritten 的请求类错误（不得按上游 5xx 罚渠道健康、不得换渠道重试）。
func TestHandleStreamToOpenAI_SafetyBlock(t *testing.T) {
	a, info := openaiInboundAdaptor(true)
	rec := httptest.NewRecorder()

	_, err := a.handleStreamToOpenAI(context.Background(),
		sseResponse(strings.NewReader("data: {\"promptFeedback\":{\"blockReason\":\"SAFETY\"}}\n\n")), info, rec)
	if err == nil {
		t.Fatal("安全过滤应返回错误")
	}
	assertContentBlockedError(t, err)
	if got := info.StreamStatus.GetEndReason(); got != common.StreamEndReasonError {
		t.Errorf("StreamStatus end reason = %v, want %v", got, common.StreamEndReasonError)
	}
	// OpenAI 客户端方向的终止语义靠 [DONE]，缺失会让客户端一直挂着等下一帧
	if !strings.Contains(rec.Body.String(), "[DONE]") {
		t.Errorf("必须以 [DONE] 收尾, got %q", rec.Body.String())
	}
}
