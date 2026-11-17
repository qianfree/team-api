package relaykit_bridge

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/helper"
)

// chat 上游 → Claude / Responses 客户端的收尾语义：上游异常结束时客户端必须拿到协议原生的
// 终止/错误事件，而不是「静默关流」或「假装正常结束」——后者正是 Claude Code / Codex
// 表现为"回答到一半就停了"的根因。

// newChatUpstreamInfo 构造「OpenAI chat 上游 → clientFormat 客户端」的桥接测试 RelayInfo。
func newChatUpstreamInfo(clientFormat constant.RelayFormat) *common.RelayInfo {
	return &common.RelayInfo{
		RequestID:       "test-req-terminal",
		OriginModelName: "gpt-4o",
		ClientFormat:    clientFormat,
		InboundFormat:   clientFormat,
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			UpstreamModelName: "gpt-4o",
		},
	}
}

func relayStatusCode(t *testing.T, err error) int {
	t.Helper()
	var relayErr *constant.RelayError
	if !errors.As(err, &relayErr) {
		t.Fatalf("err = %T %v, want *constant.RelayError", err, err)
	}
	return relayErr.StatusCode
}

// TestConvertStreamViaRelaykit_ClaudeClientEmbeddedError 上游 200+SSE 夹带错误对象：
// Claude 客户端必须收到 error 事件（此前转换失败时 Claude 方向什么都不补，连接直接关闭），
// 且不得出现 message_stop；宿主按上游故障（502 + ResponseWritten）记账。
func TestConvertStreamViaRelaykit_ClaudeClientEmbeddedError(t *testing.T) {
	sse := `data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hel"}}]}

data: {"error":{"message":"upstream rate limited","type":"rate_limit_error"}}

`
	info := newChatUpstreamInfo(constant.RelayFormatClaude)
	rec := httptest.NewRecorder()

	usage, ok, err := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(sse), rec)
	if !ok {
		t.Fatal("expected ok=true (handled)")
	}
	if err == nil || errors.Is(err, common.ErrStreamInterrupted) {
		t.Fatalf("err = %v, want upstream error (not interrupted)", err)
	}
	if !constant.IsResponseWritten(err) {
		t.Error("err should carry ResponseWritten (SSE already committed)")
	}
	if relayStatusCode(t, err) != 502 {
		t.Errorf("status = %d, want 502", relayStatusCode(t, err))
	}
	if usage == nil {
		t.Fatal("usage must not be nil")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: error") || !strings.Contains(body, `"type":"api_error"`) || !strings.Contains(body, "upstream rate limited") {
		t.Errorf("Claude client must receive an error event carrying the upstream message, body:\n%s", body)
	}
	if strings.Contains(body, "message_stop") || strings.Contains(body, "[DONE]") {
		t.Errorf("must not fake a normal end, body:\n%s", body)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonError {
		t.Errorf("end reason = %q, want error", reason)
	}
}

// TestConvertStreamViaRelaykit_ClaudeClientTruncated 上游既无 finish_reason 也无 [DONE] 即 EOF：
// 不得合成 message_stop（Claude Code 会把半截回答当完整答案），补发 error 事件；
// 转换器经侧信道上报的 usage 必须被桥接层捕获（Data 为空的事件此前在捕获前就被早退丢弃）。
func TestConvertStreamViaRelaykit_ClaudeClientTruncated(t *testing.T) {
	sse := `data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"partial answer"}}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}

`
	info := newChatUpstreamInfo(constant.RelayFormatClaude)
	rec := httptest.NewRecorder()

	usage, ok, err := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(sse), rec)
	if !ok {
		t.Fatal("expected ok=true (handled)")
	}
	if err == nil || !strings.Contains(err.Error(), "completion signal") {
		t.Fatalf("err = %v, want truncation error", err)
	}
	body := rec.Body.String()
	if strings.Contains(body, "message_stop") || strings.Contains(body, "message_delta") {
		t.Errorf("truncated stream must not synthesize message_delta/message_stop, body:\n%s", body)
	}
	if !strings.Contains(body, "event: error") {
		t.Errorf("Claude client must receive an error event, body:\n%s", body)
	}
	if usage.PromptTokens != 10 || usage.CompletionTokens != 2 {
		t.Errorf("side-channel usage not captured: %+v, want prompt=10 completion=2", usage)
	}
	// 截断记 upstream_truncated（而非笼统的 error）：属流中断，按已转发部分结算
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonUpstreamTruncated {
		t.Errorf("end reason = %q, want upstream_truncated", reason)
	}
	if !info.StreamStatus.IsPartialStreamEnd() {
		t.Error("upstream_truncated must count as partial stream end (interrupted settlement)")
	}
}

// TestConvertStreamViaRelaykit_ResponsesClientIncomplete 上游 finish_reason=length：
// Responses 客户端收到 response.incomplete 作为终止态，桥接层须将其识别为「已收尾」，
// 不得再追加 response.failed；流按正常结束记账。
func TestConvertStreamViaRelaykit_ResponsesClientIncomplete(t *testing.T) {
	sse := `data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"The answer"}}]}

data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"length"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`
	info := newChatUpstreamInfo(constant.RelayFormatResponses)
	rec := httptest.NewRecorder()

	usage, ok, err := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(sse), rec)
	if !ok {
		t.Fatal("expected ok=true (handled)")
	}
	if err != nil {
		t.Fatalf("err = %v, want nil (incomplete is a normal terminal state)", err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: response.incomplete") || !strings.Contains(body, `"reason":"max_output_tokens"`) {
		t.Errorf("Responses client must receive response.incomplete with incomplete_details, body:\n%s", body)
	}
	if strings.Contains(body, "response.failed") || strings.Contains(body, "response.completed") || strings.Contains(body, "[DONE]") {
		t.Errorf("no extra terminal event expected, body:\n%s", body)
	}
	if usage.CompletionTokens != 5 {
		t.Errorf("usage = %+v, want completion=5", usage)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonDone {
		t.Errorf("end reason = %q, want done", reason)
	}
}

// TestConvertStreamViaRelaykit_UpstreamIdleTimeout 上游发出首帧后假死：桥接路径此前没有任何
// 超时（流式 client 无 Timeout、ctx 无 deadline），客户端被 ping 维持着无限挂起。现须在
// streaming_timeout_seconds 内结束：end reason=timeout、按流中断结算（已转发文本 2 字符/token
// + 请求侧输入估算）、向 Responses 客户端补发 response.failed、以 504 上报调度。
func TestConvertStreamViaRelaykit_UpstreamIdleTimeout(t *testing.T) {
	common.SetGlobalStreamIdleTimeoutSeconds(1)
	defer common.SetGlobalStreamIdleTimeoutSeconds(0)

	pr, pw := io.Pipe()
	defer pw.Close()
	go func() {
		_, _ = io.WriteString(pw, `data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello wor"}}]}`+"\n\n")
		// 之后不再写入也不关闭：模拟上游假死
	}()

	info := newChatUpstreamInfo(constant.RelayFormatResponses)
	info.SetEstimatePromptTokens(42)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	var (
		usage *common.Usage
		ok    bool
		err   error
	)
	go func() {
		usage, ok, err = convertStreamViaRelaykit(context.Background(), info, pr, rec)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("bridge did not return after upstream idle timeout")
	}

	if !ok {
		t.Fatal("expected ok=true (handled)")
	}
	if !errors.Is(err, helper.ErrStreamIdleTimeout) {
		t.Fatalf("err = %v, want ErrStreamIdleTimeout", err)
	}
	if relayStatusCode(t, err) != 504 || !constant.IsResponseWritten(err) {
		t.Errorf("err = %v, want 504 with ResponseWritten", err)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonTimeout {
		t.Errorf("end reason = %q, want timeout", reason)
	}
	if !info.StreamStatus.IsPartialStreamEnd() {
		t.Error("timeout must count as partial stream end (interrupted settlement)")
	}
	// "Hello wor" = 9 字节 → (9+1)/2 = 5；输入用请求侧估算值补齐
	if usage.CompletionTokens != 5 || usage.PromptTokens != 42 {
		t.Errorf("usage = %+v, want completion=5 prompt=42 (interrupted fallback)", usage)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "response.output_text.delta") {
		t.Errorf("already-transferred delta should be in body:\n%s", body)
	}
	if !strings.Contains(body, "event: response.failed") || !strings.Contains(body, "idle timeout") {
		t.Errorf("Responses client must receive response.failed on upstream idle timeout, body:\n%s", body)
	}
}

// TestConvertStreamViaRelaykit_OpenAIClientTruncated Responses 上游 → OpenAI chat 客户端断流：
// 此前补发 finish_reason=stop 的终止 chunk 等于假装正常结束；现发 data 行 error 对象
// （OpenAI 官方中途失败的约定，SDK 据此抛错）+ [DONE]，记 upstream_truncated。
func TestConvertStreamViaRelaykit_OpenAIClientTruncated(t *testing.T) {
	sse := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello wor\"}\n\n"
	info := newChatUpstreamInfo(constant.RelayFormatOpenAI)
	info.UseResponsesAPI = true // chat 入站经 /v1/responses 桥接：上游格式为 Responses
	info.SetEstimatePromptTokens(42)
	rec := httptest.NewRecorder()

	usage, ok, err := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(sse), rec)
	if !ok {
		t.Fatal("expected ok=true (handled)")
	}
	if err == nil || relayStatusCode(t, err) != 502 {
		t.Fatalf("err = %v, want 502 upstream error", err)
	}
	body := rec.Body.String()
	if strings.Contains(body, `"finish_reason":"stop"`) {
		t.Errorf("must not fake finish_reason=stop on truncation, body:\n%s", body)
	}
	if !strings.Contains(body, `"code":"upstream_truncated"`) || !strings.HasSuffix(strings.TrimSpace(body), "data: [DONE]") {
		t.Errorf("client must receive an error chunk followed by [DONE], body:\n%s", body)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonUpstreamTruncated {
		t.Errorf("end reason = %q, want upstream_truncated", reason)
	}
	// "Hello wor" = 9 字节 → (9+1)/2 = 5；输入用请求侧估算值补齐
	if usage.CompletionTokens != 5 || usage.PromptTokens != 42 {
		t.Errorf("usage = %+v, want completion=5 prompt=42 (interrupted fallback)", usage)
	}
}
