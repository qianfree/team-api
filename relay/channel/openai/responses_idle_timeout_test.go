package openai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/helper"
)

// TestHandleResponsesUpstreamStream_IdleTimeout Responses 直连透传（codex 渠道 / 原生 Responses
// 上游）在上游发出首帧后假死：此前该路径直接 scanner.Scan() 读 body、没有任何超时，codex 会被
// ping 维持着无限挂起。现须在 streaming_timeout_seconds 内结束：已转发帧保留、补发
// response.failed、end reason=timeout、按流中断结算、以 504 上报调度。
func TestHandleResponsesUpstreamStream_IdleTimeout(t *testing.T) {
	common.SetGlobalStreamIdleTimeoutSeconds(1)
	defer common.SetGlobalStreamIdleTimeoutSeconds(0)

	pr, pw := io.Pipe()
	defer pw.Close()
	go func() {
		_, _ = io.WriteString(pw, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"Hel\"}\n\n")
		// 之后不再写入也不关闭：模拟上游假死
	}()

	info := &common.RelayInfo{
		RequestID:       "test-req-passthrough",
		OriginModelName: "gpt-5-codex",
		IsStream:        true,
		ClientFormat:    constant.RelayFormatResponses,
		InboundFormat:   constant.RelayFormatResponses,
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			UpstreamModelName: "gpt-5-codex",
			SupportsResponses: true,
		},
	}
	info.SetEstimatePromptTokens(42)
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: pr}
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	var (
		usage *common.Usage
		err   error
	)
	go func() {
		usage, err = (&Adaptor{}).handleResponsesUpstreamStream(context.Background(), resp, info, rec)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("passthrough did not return after upstream idle timeout")
	}

	if !errors.Is(err, helper.ErrStreamIdleTimeout) {
		t.Fatalf("err = %v, want ErrStreamIdleTimeout", err)
	}
	var relayErr *constant.RelayError
	if !errors.As(err, &relayErr) || relayErr.StatusCode != http.StatusGatewayTimeout || !relayErr.ResponseWritten {
		t.Errorf("err = %v, want 504 RelayError with ResponseWritten", err)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonTimeout {
		t.Errorf("end reason = %q, want timeout", reason)
	}
	// "Hel" = 3 字节 → (3+1)/2 = 2；输入用请求侧估算值补齐
	if usage == nil || usage.CompletionTokens != 2 || usage.PromptTokens != 42 {
		t.Errorf("usage = %+v, want completion=2 prompt=42 (interrupted fallback)", usage)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: response.output_text.delta") {
		t.Errorf("already-forwarded frame should be in body:\n%s", body)
	}
	if !strings.Contains(body, "event: response.failed") || !strings.Contains(body, "upstream_timeout") {
		t.Errorf("client must receive response.failed on upstream idle timeout, body:\n%s", body)
	}
}
