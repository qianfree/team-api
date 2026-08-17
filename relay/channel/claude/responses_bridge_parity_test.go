package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/relaykit_bridge"

	// blank import 触发内置转换器注册（与生产桥接路径一致）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// claudeSSEBody 流式对拍用 Claude SSE 报文（文本 + 工具调用 + usage 收尾）。
const claudeSSEBody = "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_01S\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-20250514\",\"usage\":{\"input_tokens\":50,\"cache_read_input_tokens\":10,\"output_tokens\":1}}}\n\n" +
	"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n" +
	"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"你好\"}}\n\n" +
	"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
	"data: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_01T\",\"name\":\"get_weather\",\"input\":{}}}\n\n" +
	"data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"city\\\":\\\"北京\\\"}\"}}\n\n" +
	"data: {\"type\":\"content_block_stop\",\"index\":1}\n\n" +
	"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"},\"usage\":{\"input_tokens\":50,\"cache_read_input_tokens\":10,\"output_tokens\":30}}\n\n" +
	"data: {\"type\":\"message_stop\"}\n\n"

// claudeNonStreamBody 非流式对拍用 Claude 响应体。
const claudeNonStreamBody = `{
	"id": "msg_01ABC", "type": "message", "role": "assistant",
	"model": "claude-sonnet-4-20250514", "stop_reason": "tool_use",
	"content": [
		{"type": "text", "text": "我先查一下天气"},
		{"type": "tool_use", "id": "toolu_01XYZ", "name": "get_weather", "input": {"city": "北京"}}
	],
	"usage": {"input_tokens": 100, "output_tokens": 40, "cache_read_input_tokens": 60, "cache_creation_input_tokens": 20}
}`

func parityRelayInfo() *common.RelayInfo {
	return &common.RelayInfo{
		IsStream:        false,
		InboundFormat:   constant.RelayFormatResponses,
		StreamStatus:    common.NewStreamStatus(),
		OriginModelName: "claude-sonnet-4",
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderClaude),
			IsModelMapped:     false,
			UpstreamModelName: "claude-sonnet-4-20250514",
		},
	}
}

// TestNonStreamToResponses_Parity 对拍 legacy handleNonStreamToResponses 与 relaykit
// TryConvertResponsesResponseViaRelaykit 的响应体。
// 已知差异（比较时排除）：created_at/completed_at 时间戳；usage 细分的零值键序列化形态
//（legacy 显式输出零值、relaykit 省略零值键，数值一致，另行断言）。
func TestNonStreamToResponses_Parity(t *testing.T) {
	// legacy 侧：经 httptest Recorder 捕获写出体
	legacyInfo := parityRelayInfo()
	legacyResp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(claudeNonStreamBody))}
	recorder := newParityRecorder(t)
	adaptor := &Adaptor{}
	legacyUsage, err := adaptor.handleNonStreamToResponses(context.Background(), legacyResp, legacyInfo, recorder.rec)
	if err != nil {
		t.Fatalf("legacy handleNonStreamToResponses: %v", err)
	}

	// relaykit 侧
	kitInfo := parityRelayInfo()
	kitBody, kitUsage, ok := relaykit_bridge.TryConvertResponsesResponseViaRelaykit(context.Background(), kitInfo, []byte(claudeNonStreamBody))
	if !ok {
		t.Fatal("expected relaykit responses converter to take over")
	}

	var legacyMap, kitMap map[string]any
	if err := json.Unmarshal(recorder.bodyBytes(), &legacyMap); err != nil {
		t.Fatalf("unmarshal legacy body: %v", err)
	}
	if err := json.Unmarshal(kitBody, &kitMap); err != nil {
		t.Fatalf("unmarshal relaykit body: %v", err)
	}
	// 排除时间戳类与 usage 细分序列化形态差异（usage 数值另行断言）
	for _, key := range []string{"created_at", "completed_at", "usage"} {
		delete(legacyMap, key)
		delete(kitMap, key)
	}
	if !reflect.DeepEqual(legacyMap, kitMap) {
		t.Errorf("parity mismatch\nlegacy:  %s\nrelaykit: %s", recorder.bodyBytes(), kitBody)
	}

	// usage 数值：客户端可见口径（OpenAI 语义）两侧一致
	if legacyUsage.PromptTokens != 180 || legacyUsage.CompletionTokens != 40 || legacyUsage.TotalTokens != 220 {
		t.Errorf("legacy visible usage = %+v, want prompt=180 completion=40 total=220", legacyUsage)
	}
	if kitUsage.PromptTokens != 180 || kitUsage.CompletionTokens != 40 || kitUsage.TotalTokens != 220 {
		t.Errorf("relaykit visible usage = %+v, want prompt=180 completion=40 total=220", kitUsage)
	}
}

// TestStreamToResponses_Parity 对拍 legacy handleStreamToResponses 与 relaykit
// TryConvertResponsesStreamViaRelaykit 的 SSE 事件序列。
// 排除时间戳生成的 id/created_at；事件类型序列与 payload 深度相等。
func TestStreamToResponses_Parity(t *testing.T) {
	// legacy 侧
	legacyInfo := parityRelayInfo()
	legacyInfo.IsStream = true
	legacyResp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(claudeSSEBody))}
	recorder := newParityRecorder(t)
	adaptor := &Adaptor{}
	legacyUsage, err := adaptor.handleStreamToResponses(context.Background(), legacyResp, legacyInfo, recorder.rec)
	if err != nil {
		t.Fatalf("legacy handleStreamToResponses: %v", err)
	}

	// relaykit 侧
	kitInfo := parityRelayInfo()
	kitInfo.IsStream = true
	kitRecorder := newParityRecorder(t)
	kitUsage, ok, streamErr := relaykit_bridge.TryConvertResponsesStreamViaRelaykit(
		context.Background(), kitInfo, io.NopCloser(strings.NewReader(claudeSSEBody)), kitRecorder.rec)
	if !ok {
		t.Fatal("expected relaykit stream converter to take over")
	}
	if streamErr != nil {
		t.Fatalf("relaykit stream conversion error: %v", streamErr)
	}

	legacyEvents := parseResponsesSSE(t, recorder.bodyBytes())
	kitEvents := parseResponsesSSE(t, kitRecorder.bodyBytes())
	if len(legacyEvents) != len(kitEvents) {
		t.Fatalf("event count mismatch: legacy=%d relaykit=%d\nlegacy:  %s\nrelaykit: %s",
			len(legacyEvents), len(kitEvents), recorder.bodyBytes(), kitRecorder.bodyBytes())
	}
	for i, le := range legacyEvents {
		ke := kitEvents[i]
		if le.eventType != ke.eventType {
			t.Fatalf("event %d type mismatch: legacy=%s relaykit=%s", i, le.eventType, ke.eventType)
		}
		normalizeEventTimestamps(le.payload)
		normalizeEventTimestamps(ke.payload)
		if !reflect.DeepEqual(le.payload, ke.payload) {
			t.Errorf("event %d (%s) payload mismatch\nlegacy:  %v\nrelaykit: %v", i, le.eventType, le.payload, ke.payload)
		}
	}

	// 计费 usage：旧路径 Claude 口径（input 不含缓存），relaykit OpenAI 口径（含缓存）。
	// 金额等价（input 50 + cache_read 10 = prompt 60）：断言语义换算成立
	if legacyUsage.PromptTokens != 50 {
		t.Errorf("legacy billing prompt tokens = %d, want 50 (Claude 口径不含缓存)", legacyUsage.PromptTokens)
	}
	if kitUsage.PromptTokens != 60 {
		t.Errorf("relaykit billing prompt tokens = %d, want 60 (OpenAI 口径含缓存)", kitUsage.PromptTokens)
	}
	if legacyUsage.CompletionTokens != kitUsage.CompletionTokens {
		t.Errorf("completion tokens mismatch: legacy=%d relaykit=%d", legacyUsage.CompletionTokens, kitUsage.CompletionTokens)
	}
}
