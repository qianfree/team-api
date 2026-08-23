package relaykit_bridge

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
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

// TestConvertStreamViaRelaykit_EstimatedUsageOnMissingUsage 上游流未携带 usage 时
// （如部分上游正常结束但末事件不带用量），正常结束按已转发文本 4 字符/token 估算兜底。
func TestConvertStreamViaRelaykit_EstimatedUsageOnMissingUsage(t *testing.T) {
	claudeStream := `data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-3-opus-20240229"}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello, how can I help you?"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}

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
	// "Hello, how can I help you?" 共 26 字符，正常结束 4 字符/token → 26/4 = 6
	if usage.CompletionTokens != 6 {
		t.Errorf("CompletionTokens = %d, want 6 (estimated from 26 chars / 4)", usage.CompletionTokens)
	}
	if usage.TotalTokens != 6 {
		t.Errorf("TotalTokens = %d, want 6 (prompt=0 + estimated 6)", usage.TotalTokens)
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

// TestConvertStreamViaRelaykit_ResponsesToChatToolCallItemIDMismatch 回归：
// ChatViaResponses 渠道（chat 客户端 × responses 上游）流式工具调用的 item_id/call_id
// 键归一。Responses 上游的 response.function_call_arguments.delta 只携带 item_id
// （output item 的 id，如 fc_xxx，≠ call_id call_xxx）——不归一到同一键时 delta 事件
// 分配新 index，name 与参数碎裂成两个 tool_call，done 事件再按首个 index 全量重发参数，
// 客户端组装出非法 JSON。（移植自闭主的 HandleResponsesStreamToChat 回归测试，随双实现收割。）
func TestConvertStreamViaRelaykit_ResponsesToChatToolCallItemIDMismatch(t *testing.T) {
	ss := strings.Join([]string{
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","item":{"id":"fc_1","call_id":"call_1","type":"function_call","name":"lookup","arguments":""}}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"a\""}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":":1}"}`,
		"",
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","item":{"id":"fc_1","call_id":"call_1","type":"function_call","name":"lookup","arguments":"{\"a\":1}"}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":5,"output_tokens":7,"total_tokens":12}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	info := newStreamTestRelayInfo(constant.ProviderOpenAI, constant.RelayFormatOpenAI)
	info.UseResponsesAPI = true // ChatViaResponses：bridgeUpstreamFormat 判定上游为 responses
	info.ChannelMeta.UpstreamModelName = "gpt-5-codex"
	rec := httptest.NewRecorder()

	usage, ok := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(ss), rec)
	if !ok {
		t.Fatal("expected ok=true (handled), got false")
	}

	// 解析输出 SSE，聚合 tool_calls 的 index 与参数
	indexes := map[int]bool{}
	argsByID := map[int]string{}
	namesByID := map[int]string{}
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") || strings.Contains(line, "[DONE]") {
			continue
		}
		var chunk dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(line[len("data: "):]), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			for _, tc := range choice.Delta.ToolCalls {
				indexes[tc.Index] = true
				argsByID[tc.Index] += tc.Function.Arguments
				if tc.Function.Name != "" {
					namesByID[tc.Index] = tc.Function.Name
				}
			}
		}
	}

	if len(indexes) != 1 {
		t.Errorf("tool_call index 集合 = %v, want 单一 index（delta 未归一到 call_id 键则碎裂成两个）", indexes)
	}
	for idx, args := range argsByID {
		if want := `{"a":1}`; args != want {
			t.Errorf("index %d 参数拼接 = %q, want %q（done 全量重发会拼出重复参数）", idx, args, want)
		}
	}
	for idx, name := range namesByID {
		if name != "lookup" {
			t.Errorf("index %d name = %q, want lookup", idx, name)
		}
	}
	if len(namesByID) == 0 {
		t.Error("未发出任何带 name 的 tool_call chunk")
	}

	// usage 取自 response.completed 的独立 usage chunk（OpenAI include_usage 语义）
	if usage == nil || usage.PromptTokens != 5 || usage.CompletionTokens != 7 || usage.TotalTokens != 12 {
		t.Errorf("usage = %+v, want prompt=5 completion=7 total=12", usage)
	}
	if !strings.HasSuffix(strings.TrimSpace(rec.Body.String()), "data: [DONE]") {
		t.Errorf("output should end with [DONE], got tail: %q", tail(rec.Body.String(), 40))
	}
	if info.StreamStatus == nil || info.StreamStatus.GetEndReason() != common.StreamEndReasonDone {
		t.Errorf("expected end reason %q, got %v", common.StreamEndReasonDone, info.StreamStatus.GetEndReason())
	}
}

// tail 返回 s 末尾最多 n 字节（用于错误断言展示）。
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// TestConvertStreamViaRelaykit_OpenAIToGemini_NormalCompletion openai 上游 → gemini 客户端：
// 正常结束以转换器带 finishReason 的尾 chunk 收尾，不写 [DONE]（官方 Gemini SSE 无此哨兵），
// 也不应漏出任何 chat 格式 chunk。
func TestConvertStreamViaRelaykit_OpenAIToGemini_NormalCompletion(t *testing.T) {
	openaiStream := `data: {"id":"c1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}

data: {"id":"c1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello!"}}]}

data: {"id":"c1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":5,"total_tokens":13}}

data: [DONE]

`
	info := newStreamTestRelayInfo(constant.ProviderOpenAI, constant.RelayFormatGemini)
	rec := httptest.NewRecorder()

	usage, ok := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(openaiStream), rec)
	if !ok {
		t.Fatal("expected ok=true (handled), got false")
	}
	if usage == nil || usage.PromptTokens != 8 || usage.CompletionTokens != 5 {
		t.Errorf("usage = %+v, want prompt=8 completion=5", usage)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"text":"Hello!"`) {
		t.Errorf("output missing streamed text, got: %s", body)
	}
	if !strings.Contains(body, `"finishReason":"STOP"`) {
		t.Errorf("output missing finishReason=STOP tail chunk, got: %s", body)
	}
	if strings.Contains(body, "[DONE]") {
		t.Errorf("output should not contain [DONE] for gemini client, got tail: %q", tail(body, 60))
	}
	if strings.Contains(body, "chat.completion.chunk") {
		t.Error("output should not contain chat-format chunks for gemini client")
	}
	if info.StreamStatus == nil || info.StreamStatus.GetEndReason() != common.StreamEndReasonDone {
		t.Errorf("expected end reason %q, got %v", common.StreamEndReasonDone, info.StreamStatus.GetEndReason())
	}
}

// TestConvertStreamViaRelaykit_OpenAIToGemini_TruncatedStreamTerminal 截断流（上游无
// finish_reason 即 EOF）时按 Gemini 格式补终止 chunk（finishReason=STOP），不写 [DONE]、
// 不漏出 chat 格式终止 chunk。
func TestConvertStreamViaRelaykit_OpenAIToGemini_TruncatedStreamTerminal(t *testing.T) {
	openaiStream := `data: {"id":"c1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hi"}}]}

`
	info := newStreamTestRelayInfo(constant.ProviderOpenAI, constant.RelayFormatGemini)
	rec := httptest.NewRecorder()

	_, ok := convertStreamViaRelaykit(context.Background(), info, strings.NewReader(openaiStream), rec)
	if !ok {
		t.Fatal("expected ok=true (handled), got false")
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"text":"Hi"`) {
		t.Errorf("output missing streamed text, got: %s", body)
	}
	if !strings.Contains(body, `"finishReason":"STOP"`) {
		t.Errorf("output missing gemini-format terminal chunk, got: %s", body)
	}
	if strings.Contains(body, "[DONE]") {
		t.Errorf("output should not contain [DONE] for gemini client, got tail: %q", tail(body, 60))
	}
	if strings.Contains(body, "chat.completion.chunk") {
		t.Error("output should not contain chat-format terminal chunk for gemini client")
	}
}
