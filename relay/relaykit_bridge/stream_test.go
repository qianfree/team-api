package relaykit_bridge

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	// blank import 触发内置流式转换器注册（register.init() → RegisterStreamConverter）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// newStreamTestRelayInfo 构造流式桥接测试用的 RelayInfo。
func newStreamTestRelayInfo(channelType constant.ProviderType, clientFormat constant.RelayFormat) *common.RelayInfo {
	return &common.RelayInfo{
		RequestID:       "test-req-stream",
		OriginModelName: "gpt-4",
		ClientFormat:    clientFormat,
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(channelType),
			UpstreamModelName: "claude-3-opus-20240229",
		},
	}
}

// TestConvertStreamViaRelaykit_ClaudeToOpenAI 验证 Claude SSE 流经 relaykit 转换为 OpenAI SSE，
// 并正确提取 usage、写入 [DONE] 收尾、设置正常结束原因。
func TestConvertStreamViaRelaykit_ClaudeToOpenAI(t *testing.T) {
	// 复用 relaykit 内 claude_to_openai_stream_test.go 的 BasicStream 报文
	claudeStream := `data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-3-opus-20240229","usage":{"input_tokens":10,"output_tokens":0}}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":", how can I help you?"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}

data: {"type":"message_stop"}

`

	info := newStreamTestRelayInfo(constant.ProviderClaude, constant.RelayFormatOpenAI)
	rec := httptest.NewRecorder()

	usage, ok := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(claudeStream), rec)
	if !ok {
		t.Fatal("expected ok=true (handled), got false")
	}
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}
	if usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", usage.PromptTokens)
	}
	if usage.CompletionTokens != 7 {
		t.Errorf("CompletionTokens = %d, want 7", usage.CompletionTokens)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "chat.completion.chunk") {
		t.Error("output missing chat.completion.chunk object")
	}
	if !strings.Contains(body, "Hello") || !strings.Contains(body, "how can I help you?") {
		t.Errorf("output missing streamed text, got: %s", body)
	}
	if !strings.Contains(body, `"usage"`) {
		t.Error("output missing final usage chunk")
	}
	if !strings.HasSuffix(strings.TrimSpace(body), "data: [DONE]") {
		t.Errorf("output should end with [DONE], got tail: %q", tail(body, 40))
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", rec.Header().Get("Content-Type"))
	}
	if info.StreamStatus == nil || info.StreamStatus.GetEndReason() != common.StreamEndReasonDone {
		t.Errorf("expected end reason %q, got %v", common.StreamEndReasonDone, info.StreamStatus.GetEndReason())
	}
}

// TestConvertStreamViaRelaykit_SameFormatFallback 同格式（OpenAI→OpenAI）无需转换，应回退（ok=false）。
func TestConvertStreamViaRelaykit_SameFormatFallback(t *testing.T) {
	info := newStreamTestRelayInfo(constant.ProviderOpenAI, constant.RelayFormatOpenAI)
	rec := httptest.NewRecorder()

	usage, ok := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(""), rec)
	if ok {
		t.Fatal("expected ok=false for same format, got true")
	}
	if usage != nil {
		t.Errorf("expected nil usage on fallback, got %+v", usage)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected no output on fallback, got %q", rec.Body.String())
	}
}

// TestConvertStreamViaRelaykit_NoMatchingRoute 无匹配流式转换器的方向（OpenAI→Gemini）应回退。
func TestConvertStreamViaRelaykit_NoMatchingRoute(t *testing.T) {
	info := newStreamTestRelayInfo(constant.ProviderOpenAI, constant.RelayFormatGemini)
	rec := httptest.NewRecorder()

	usage, ok := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(""), rec)
	if ok {
		t.Fatal("expected ok=false for unmatched route, got true")
	}
	if usage != nil {
		t.Errorf("expected nil usage on fallback, got %+v", usage)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected no output on fallback, got %q", rec.Body.String())
	}
}

// TestTryConvertStreamViaRelaykit_NilGuards 公开入口对 nil info / 无 ChannelMeta 应安全回退。
// （nil 守卫位于公开入口；core convertStreamViaRelaykit 由调用方保证 info 非空。）
func TestTryConvertStreamViaRelaykit_NilGuards(t *testing.T) {
	rec := httptest.NewRecorder()
	if _, ok := TryConvertStreamViaRelaykit(context.Background(), nil, strings.NewReader(""), rec); ok {
		t.Fatal("expected ok=false for nil info")
	}

	info := newStreamTestRelayInfo(constant.ProviderClaude, constant.RelayFormatOpenAI)
	info.ChannelMeta = nil
	if _, ok := TryConvertStreamViaRelaykit(context.Background(), info, strings.NewReader(""), rec); ok {
		t.Fatal("expected ok=false for nil ChannelMeta")
	}
}

// tail 返回 s 末尾最多 n 字节（用于错误断言展示）。
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
