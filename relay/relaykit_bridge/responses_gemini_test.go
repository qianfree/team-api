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

	// blank import 触发内置转换器注册（与生产桥接路径一致）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// newGeminiUpstreamInfo 构造 gemini 上游 × responses 客户端的测试 RelayInfo。
func newGeminiUpstreamInfo() *common.RelayInfo {
	return &common.RelayInfo{
		RequestID:       "test-req-g2r",
		OriginModelName: "gemini-2.0-flash",
		ClientFormat:    constant.RelayFormatResponses,
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderGemini),
			UpstreamModelName: "gemini-2.0-flash",
		},
	}
}

// TestResponsesStreamViaRelaykit_GeminiUpstream 回归 Gap A：gemini 上流式 SSE → responses
// 客户端事件流。此前 responses 桥接 gate 无 gemini 分支，回退 stream.go 桥接命中同一条
// io.Pipe 组合链但其 chunkWriter 不认 ResponsesStreamEvent——事件被静默丢弃，客户端只
// 收到 chat 格式收尾。修复后 gate 放行，由 emitResponsesEvent 正确输出事件与 usage。
func TestResponsesStreamViaRelaykit_GeminiUpstream(t *testing.T) {
	geminiStream := `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"index":0}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":" world"}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}

`
	info := newGeminiUpstreamInfo()
	rec := httptest.NewRecorder()

	usage, ok, streamErr := TryConvertResponsesStreamViaRelaykit(context.Background(), info, strings.NewReader(geminiStream), rec)
	if !ok {
		t.Fatal("expected ok=true（gemini→responses 流式组合链已注册，gate 应放行）")
	}
	if streamErr != nil {
		t.Fatalf("unexpected stream error: %v", streamErr)
	}

	body := rec.Body.String()
	// 事件未被丢弃：完整 Responses 事件序列（created → output_text.delta → completed）
	for _, want := range []string{"event: response.created", "event: response.output_text.delta", "event: response.completed"} {
		if !strings.Contains(body, want) {
			t.Errorf("输出缺少事件 %q；实际输出:\n%s", want, body)
		}
	}
	if !strings.Contains(body, "hello") {
		t.Errorf("输出缺少文本内容；实际输出:\n%s", body)
	}
	if strings.Contains(body, "chat.completion.chunk") {
		t.Errorf("不应出现 chat 格式 chunk（收尾须为 Responses 事件）；实际输出:\n%s", body)
	}
	// usage 从 response.completed 提取（OpenAI 口径：input=10、output=5）
	if usage == nil || usage.PromptTokens != 10 || usage.CompletionTokens != 5 {
		t.Errorf("usage = %+v, want prompt=10 completion=5", usage)
	}
}

// TestResponsesResponseViaRelaykit_GeminiUpstream gemini 上游非流式 → responses 客户端
// （gate 与 parseUpstreamResponse 的 gemini 分支一致性：组合 Resp g2o→o2r + usage 提取）。
func TestResponsesResponseViaRelaykit_GeminiUpstream(t *testing.T) {
	geminiBody := `{"candidates":[{"content":{"role":"model","parts":[{"text":"hello world"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15},"modelVersion":"gemini-2.0-flash"}`
	info := newGeminiUpstreamInfo()

	converted, usage, ok := TryConvertResponsesResponseViaRelaykit(context.Background(), info, []byte(geminiBody))
	if !ok {
		t.Fatal("expected ok=true（gemini→responses 响应组合已注册）")
	}

	var resp dto.OpenAIResponsesResponse
	if err := json.Unmarshal(converted, &resp); err != nil {
		t.Fatalf("converted body is not valid responses JSON: %v", err)
	}
	if resp.Object != "response" {
		t.Errorf("object = %q, want %q", resp.Object, "response")
	}
	if usage == nil || usage.PromptTokens != 10 || usage.CompletionTokens != 5 {
		t.Errorf("usage = %+v, want prompt=10 completion=5", usage)
	}
}
