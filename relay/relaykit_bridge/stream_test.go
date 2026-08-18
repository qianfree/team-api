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
	claudeStream := `data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-3-opus-20240229","usage":{"input_tokens":10,"output_tokens":0,"cache_read_input_tokens":4,"cache_creation_input_tokens":3}}}

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
	// 转换器已按 OpenAI 口径做加法：prompt = input(10) + cache_read(4) + cache_creation(3)
	if usage.PromptTokens != 17 {
		t.Errorf("PromptTokens = %d, want 17", usage.PromptTokens)
	}
	if usage.CompletionTokens != 7 {
		t.Errorf("CompletionTokens = %d, want 7", usage.CompletionTokens)
	}
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 4 || usage.CacheCreationTokens != 3 {
		t.Errorf("cache usage = %+v, want read=4 creation=3", usage)
	}
	// 转换后的 prompt 已含缓存（OpenAI 子集语义），计费前须扣减缓存部分避免双重计费
	if !usage.CacheIncludedInPrompt {
		t.Error("CacheIncludedInPrompt = false, want true for Claude upstream (converted usage is OpenAI semantics)")
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

// TestConvertStreamViaRelaykit_GeminiToOpenAI 验证 Gemini SSE 流经 relaykit 转换为 OpenAI SSE，
// 缓存/思考 token 明细正确透出，且因 Gemini 的 promptTokenCount 已含 cached（子集语义）
// 计费标记 CacheIncludedInPrompt=true（计费时扣减缓存部分，避免双重计费）。
func TestConvertStreamViaRelaykit_GeminiToOpenAI(t *testing.T) {
	// 报文格式同 relaykit golden 用例 03_basic_streaming，末 chunk 带缓存/思考用量
	geminiStream := `data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Hello!"}]}}]}

data: {"candidates":[{"index":0,"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":5,"totalTokenCount":25,"cachedContentTokenCount":8,"thoughtsTokenCount":3}}

`

	info := newStreamTestRelayInfo(constant.ProviderGemini, constant.RelayFormatOpenAI)
	rec := httptest.NewRecorder()

	usage, ok := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(geminiStream), rec)
	if !ok {
		t.Fatal("expected ok=true (handled), got false")
	}
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}
	// 转换器已按 OpenAI 口径合并思考 token：completion = candidates(5) + thoughts(3)
	if usage.PromptTokens != 20 || usage.CompletionTokens != 8 || usage.TotalTokens != 25 {
		t.Errorf("token counts = %+v, want prompt=20 completion=8 total=25", usage)
	}
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 8 {
		t.Errorf("cached tokens = %+v, want 8", usage.PromptTokensDetails)
	}
	if usage.CompletionTokenDetails == nil || usage.CompletionTokenDetails.ReasoningTokens != 3 {
		t.Errorf("reasoning tokens = %+v, want 3", usage.CompletionTokenDetails)
	}
	if !usage.CacheIncludedInPrompt {
		t.Error("CacheIncludedInPrompt = false, want true for Gemini upstream (cached ⊆ prompt)")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Hello!") {
		t.Errorf("output missing streamed text, got: %s", body)
	}
	if !strings.HasSuffix(strings.TrimSpace(body), "data: [DONE]") {
		t.Errorf("output should end with [DONE], got tail: %q", tail(body, 40))
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

// TestConvertStreamViaRelaykit_NoMatchingRoute 无匹配流式转换器的方向（Coze→Claude——
// P2 后 OpenAI→Gemini 已注册，改用真正未注册的方向）应回退。
func TestConvertStreamViaRelaykit_NoMatchingRoute(t *testing.T) {
	info := newStreamTestRelayInfo(constant.ProviderCoze, constant.RelayFormatClaude)
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
