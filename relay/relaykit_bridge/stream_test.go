package relaykit_bridge

import (
	"context"
	"errors"
	"net/http"
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

	usage, ok, err := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(claudeStream), rec)
	if !ok {
		t.Fatal("expected ok=true (handled), got false")
	}
	if err != nil {
		t.Fatalf("expected nil err on clean stream, got %v", err)
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
	// 缓存明细：读取=4（CachedTokens）、写入=3（details.CacheWriteTokens 唯一携带点，
	// 顶层 CacheCreationTokens 恒 0——宿主落库做「顶层+details.CacheWriteTokens」相加，
	// bridge 填顶层会同源双计）
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 4 ||
		usage.PromptTokensDetails.CacheWriteTokens != 3 || usage.CacheCreationTokens != 0 {
		t.Errorf("cache usage = %+v, want read=4 write(details)=3 toplevel=0", usage)
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

// TestConvertStreamViaRelaykit_ClaudeToResponses_BillingUsageSemantics
// 回归（双重扣减）：Claude→Responses 流式的计费侧信道必须为含缓存口径。
// 修复前转换器终帧发 Claude 口径（prompt 不含缓存），而桥接层对侧信道硬编码
// CacheIncludedInPrompt=true，计费 resolveTokenCounts 再扣一次缓存明细 →
// baseInput = input − cacheRead − cc5m − cc1h 被错误压低（本例 1000−11000 → 钳 0，
// 输入分文未计）。修复后 prompt=12000（含缓存），扣减还原 baseInput=1000。
func TestConvertStreamViaRelaykit_ClaudeToResponses_BillingUsageSemantics(t *testing.T) {
	claudeStream := `data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5","usage":{"input_tokens":1000,"output_tokens":1,"cache_read_input_tokens":4000,"cache_creation_input_tokens":7000,"cache_creation":{"ephemeral_5m_input_tokens":3000,"ephemeral_1h_input_tokens":4000}}}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}

data: {"type":"message_stop"}

`
	info := newStreamTestRelayInfo(constant.ProviderClaude, constant.RelayFormatResponses)
	rec := httptest.NewRecorder()

	usage, ok, err := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(claudeStream), rec)
	if !ok || err != nil {
		t.Fatalf("expected (true, nil), got (%v, %v)", ok, err)
	}
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}
	// 侧信道口径：prompt 含缓存 = 1000 + 4000 + 7000
	if usage.PromptTokens != 12000 {
		t.Errorf("PromptTokens = %d, want 12000（含缓存；不含缓存会被双重扣减）", usage.PromptTokens)
	}
	if !usage.CacheIncludedInPrompt {
		t.Error("CacheIncludedInPrompt = false, want true")
	}
	d := usage.PromptTokensDetails
	if d == nil || d.CachedTokens != 4000 || d.CachedCreation5mTokens != 3000 || d.CachedCreation1hTokens != 4000 {
		t.Fatalf("cache details = %+v, want read=4000 cc5m=3000 cc1h=4000", d)
	}
	// 计费扣减数值推演（与 billing.resolveTokenCounts 同式）：
	// baseInput = 12000 − 4000 − 3000 − 4000 = 1000（修复前 = 0，输入全免）
	baseInput := usage.PromptTokens - d.CachedTokens - d.CachedCreation5mTokens - d.CachedCreation1hTokens
	if baseInput != 1000 {
		t.Errorf("baseInput 推演 = %d, want 1000", baseInput)
	}
}

// TestConvertStreamViaRelaykit_ClaudeToGemini_BillingUsageSemantics
// 回归（双重扣减）：Claude→Gemini 流式同款口径约束，见 Responses 方向用例说明
func TestConvertStreamViaRelaykit_ClaudeToGemini_BillingUsageSemantics(t *testing.T) {
	claudeStream := `data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5","usage":{"input_tokens":40,"output_tokens":1,"cache_read_input_tokens":10}}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":20}}

data: {"type":"message_stop"}

`
	info := newStreamTestRelayInfo(constant.ProviderClaude, constant.RelayFormatGemini)
	rec := httptest.NewRecorder()

	usage, ok, err := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(claudeStream), rec)
	if !ok || err != nil {
		t.Fatalf("expected (true, nil), got (%v, %v)", ok, err)
	}
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}
	// 侧信道口径：prompt 含缓存 = 40 + 10（无 cache_creation）
	if usage.PromptTokens != 50 {
		t.Errorf("PromptTokens = %d, want 50（含缓存）", usage.PromptTokens)
	}
	if !usage.CacheIncludedInPrompt {
		t.Error("CacheIncludedInPrompt = false, want true")
	}
	d := usage.PromptTokensDetails
	if d == nil || d.CachedTokens != 10 {
		t.Fatalf("cache details = %+v, want read=10", d)
	}
	// baseInput = 50 − 10 = 40（修复前 prompt=40 → 扣减后 30，少计 10）
	if base := usage.PromptTokens - d.CachedTokens; base != 40 {
		t.Errorf("baseInput 推演 = %d, want 40", base)
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

	usage, ok, err := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(geminiStream), rec)
	if !ok {
		t.Fatal("expected ok=true (handled), got false")
	}
	if err != nil {
		t.Fatalf("expected nil err on clean stream, got %v", err)
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

	usage, ok, err := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(""), rec)
	if err != nil {
		t.Fatalf("expected nil err on fallback, got %v", err)
	}
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

// TestConvertStreamViaRelaykit_NoMatchingRoute 无匹配流式转换器的方向应回退。
// Ollama 上游 → Claude 客户端未注册（Ollama 仅覆盖 OpenAI 客户端方向）。
func TestConvertStreamViaRelaykit_NoMatchingRoute(t *testing.T) {
	info := newStreamTestRelayInfo(constant.ProviderOllama, constant.RelayFormatClaude)
	rec := httptest.NewRecorder()

	usage, ok, err := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(""), rec)
	if err != nil {
		t.Fatalf("expected nil err on fallback, got %v", err)
	}
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
	if _, ok, _ := TryConvertStreamViaRelaykit(context.Background(), nil, strings.NewReader(""), rec); ok {
		t.Fatal("expected ok=false for nil info")
	}

	info := newStreamTestRelayInfo(constant.ProviderClaude, constant.RelayFormatOpenAI)
	info.ChannelMeta = nil
	if _, ok, _ := TryConvertStreamViaRelaykit(context.Background(), info, strings.NewReader(""), rec); ok {
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

// failOnSubstringWriter 写入载荷包含 marker 时返回错误（模拟客户端已断开后的写失败），
// 其余写入透传到内层 recorder。
type failOnSubstringWriter struct {
	rec    *httptest.ResponseRecorder
	marker string
}

func (w *failOnSubstringWriter) Header() http.Header  { return w.rec.Header() }
func (w *failOnSubstringWriter) WriteHeader(code int) { w.rec.WriteHeader(code) }
func (w *failOnSubstringWriter) Write(p []byte) (int, error) {
	if strings.Contains(string(p), w.marker) {
		return 0, errors.New("write failed: connection reset by peer")
	}
	return w.rec.Body.Write(p)
}

// TestConvertStreamViaRelaykit_ClientWriteFailure 复现中断误判竞态：客户端断开由「写失败」
// 首先暴露而 ctx 尚未取消（断开信号经代理传导延迟/first-writer-wins 场景）。
// 此时必须按流中断处理：end reason=client_gone、usage 走中断兜底（已转发文本 2 字符/token
// 估算），且不向死连接补写 [DONE]——而非误判为转换失败导致 0 token / 0 费用结算。
func TestConvertStreamViaRelaykit_ClientWriteFailure(t *testing.T) {
	claudeStream := `data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-3-opus-20240229","usage":{"input_tokens":10,"output_tokens":0,"cache_read_input_tokens":4,"cache_creation_input_tokens":3}}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":", how can I help you?"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}

data: {"type":"message_stop"}

`
	info := newStreamTestRelayInfo(constant.ProviderClaude, constant.RelayFormatOpenAI)
	// 与生产一致（relay_handler buildRelayInfo 设置）：流中断时上游 usage 缺失，
	// 输入 token 按请求侧估算值补齐计费
	info.SetEstimatePromptTokens(42)
	// ctx 用 Background（永不取消），精确模拟「写失败先于 ctx 取消被观察到」的竞态
	w := &failOnSubstringWriter{rec: httptest.NewRecorder(), marker: "how can I help you"}

	usage, ok, err := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(claudeStream), w)
	if !ok {
		t.Fatal("expected ok=true (handled), got false")
	}
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}
	// 中断信号必须由桥接层直接透出：调用方据此按流中断结算，不再自行读 StreamStatus 判定
	if !errors.Is(err, common.ErrStreamInterrupted) {
		t.Fatalf("err = %v, want common.ErrStreamInterrupted", err)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonClientGone {
		t.Fatalf("end reason = %q, want %q (client write failure misclassified)", reason, common.StreamEndReasonClientGone)
	}
	if !info.StreamStatus.IsPartialStreamEnd() {
		t.Fatal("IsPartialStreamEnd = false, want true")
	}
	// 转换器的 usage chunk 只在流末尾产出，中断时尚未捕获——prompt 按请求侧估算值(42)补齐；
	// 两个 delta 文本（"Hello"+", how can I help you?" = 5+21 = 26 字节）均在写失败前计入
	// transferredTextLen（累计发生在写之前），中断估算 completion = (26+1)/2 = 13
	if usage.PromptTokens != 42 {
		t.Errorf("PromptTokens = %d, want 42 (request-side estimate fallback)", usage.PromptTokens)
	}
	if usage.CompletionTokens != 13 {
		t.Errorf("CompletionTokens = %d, want 13 (interrupted estimate, got 0 => fallback skipped)", usage.CompletionTokens)
	}
	// 中断后不得向死连接补写 [DONE]
	if strings.Contains(w.rec.Body.String(), "[DONE]") {
		t.Error("output should not contain [DONE] after client disconnect")
	}
}
