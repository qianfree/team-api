package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// responsesInboundInfo 构造 responses 入站（codex 等）+ Claude 上游渠道的 RelayInfo
func responsesInboundInfo(isStream bool) *common.RelayInfo {
	return &common.RelayInfo{
		RelayMode:       int(constant.RelayModeResponses),
		IsStream:        isStream,
		InboundFormat:   constant.RelayFormatResponses,
		ClientFormat:    constant.RelayFormatResponses,
		OriginModelName: "glm-5.3",
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			BaseURL:           "https://upstream.example.com",
			// ChannelType 必须显式设置：ProviderNativeFormat(0) 默认归为 openai，
			// 会让 relaykit 桥接把 Claude 格式响应误解析为 chat（空 choices → 空输出）
			ChannelType:       int(constant.ProviderClaude),
			UpstreamModelName: "glm-5.3",
			IsModelMapped:     false,
		},
	}
}

// TestGetRequestURL_ResponsesMode responses/compact 模式同样打 /v1/messages
// （请求侧转 Claude Messages 格式，响应侧转回 Responses）。
func TestGetRequestURL_ResponsesMode(t *testing.T) {
	for _, mode := range []constant.RelayMode{constant.RelayModeResponses, constant.RelayModeResponsesCompact} {
		a := &Adaptor{}
		info := responsesInboundInfo(false)
		info.RelayMode = int(mode)
		got, err := a.GetRequestURL(info)
		if err != nil {
			t.Fatalf("GetRequestURL(%v) error: %v", mode, err)
		}
		if want := "https://upstream.example.com/v1/messages"; got != want {
			t.Errorf("GetRequestURL(%v) = %q, want %q", mode, got, want)
		}
	}
}

// TestDoResponse_ResponsesInboundStream_TextAndToolCall
// Claude SSE（文本 + 工具调用）完整转换为 Responses SSE：
// 事件序列、工具参数聚合、completed 的 usage 映射（OpenAI 语义）。
func TestDoResponse_ResponsesInboundStream_TextAndToolCall(t *testing.T) {
	ss := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_01","model":"glm-5.3","usage":{"input_tokens":10,"output_tokens":1}}}`,
		``,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你好"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_01","name":"shell","input":{}}}`,
		``,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"cmd\":\"ls\"}"}}`,
		``,
		`data: {"type":"content_block_stop","index":1}`,
		``,
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":10,"cache_read_input_tokens":4,"output_tokens":7}}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(ss)),
	}

	info := responsesInboundInfo(true)
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	usage, err := a.DoResponse(context.Background(), resp, info, rec)
	if err != nil {
		t.Fatalf("DoResponse error: %v", err)
	}
	out := rec.Body.String()

	// 事件序列完整性
	for _, want := range []string{
		"event: response.created",
		"event: response.output_item.added",
		"event: response.content_part.added",
		`event: response.output_text.delta`,
		"event: response.output_text.done",
		"event: response.content_part.done",
		"event: response.output_item.done",
		"event: response.function_call_arguments.delta",
		"event: response.function_call_arguments.done",
		"event: response.completed",
	} {
		if !strings.Contains(out, want+"\n") {
			t.Errorf("missing event %q in output:\n%s", want, out)
		}
	}
	if !strings.Contains(out, `"delta":"你好"`) {
		t.Errorf("text delta missing:\n%s", out)
	}
	if !strings.Contains(out, `"delta":"{\"cmd\":\"ls\"}"`) {
		t.Errorf("tool arguments delta missing:\n%s", out)
	}
	if !strings.Contains(out, `"name":"shell"`) {
		t.Errorf("tool name missing:\n%s", out)
	}

	// completed 的 usage 用 OpenAI 语义：input 含缓存（10+4），cached_tokens 为子集
	if !strings.Contains(out, `"input_tokens":14`) {
		t.Errorf("completed usage input_tokens should be 14 (input+cache_read):\n%s", out)
	}
	if !strings.Contains(out, `"cached_tokens":4`) {
		t.Errorf("completed usage cached_tokens should be 4:\n%s", out)
	}
	if !strings.Contains(out, `"output_tokens":7`) {
		t.Errorf("completed usage output_tokens should be 7:\n%s", out)
	}

	// completed 的 output 数组：message + function_call
	completedIdx := strings.LastIndex(out, "event: response.completed")
	if completedIdx < 0 {
		t.Fatalf("response.completed missing:\n%s", out)
	}
	completed := out[completedIdx:]
	if !strings.Contains(completed, `"type":"function_call"`) {
		t.Errorf("completed output should contain function_call item:\n%s", completed)
	}

	// 计费返回值按 OpenAI 口径（relaykit 桥接，P0 已上线行为）：input 含缓存（10+4），
	// CacheIncludedInPrompt=true 由计费侧扣减缓存部分——金额与 Claude 口径等价
	if usage.PromptTokens != 14 || usage.CompletionTokens != 7 {
		t.Errorf("billing usage = %+v, want prompt=14 completion=7", usage)
	}
	if !usage.CacheIncludedInPrompt {
		t.Errorf("billing usage should set CacheIncludedInPrompt=true: %+v", usage)
	}
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 4 {
		t.Errorf("billing usage cache detail = %+v, want cached_tokens=4", usage.PromptTokensDetails)
	}
}

// TestDoResponse_ResponsesInboundStream_UpstreamErrorBeforeEvents
// 首个事件即 error：返回 upstream error 并写入错误体，不合成假成功的 response.completed。
func TestDoResponse_ResponsesInboundStream_UpstreamErrorBeforeEvents(t *testing.T) {
	ss := strings.Join([]string{
		`data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
		``,
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(ss)),
	}

	info := responsesInboundInfo(true)
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	_, err := a.DoResponse(context.Background(), resp, info, rec)
	if err == nil {
		t.Fatal("upstream error event should return an upstream error, got nil")
	}
	if !strings.Contains(rec.Body.String(), "overloaded_error") {
		t.Errorf("error body not written to client: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "response.completed") {
		t.Errorf("should not synthesize response.completed on upstream error: %s", rec.Body.String())
	}
}

// TestDoResponse_ResponsesInboundNonStream Claude 非流式 JSON 转换为 Responses 格式。
func TestDoResponse_ResponsesInboundNonStream(t *testing.T) {
	body := `{"id":"msg_02","type":"message","role":"assistant","model":"glm-5.3","content":[` +
		`{"type":"text","text":"答案是42"},` +
		`{"type":"tool_use","id":"toolu_02","name":"read","input":{"path":"a.go"}}` +
		`],"stop_reason":"end_turn","usage":{"input_tokens":10,"cache_read_input_tokens":4,"output_tokens":5}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	info := responsesInboundInfo(false)
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	usage, err := a.DoResponse(context.Background(), resp, info, rec)
	if err != nil {
		t.Fatalf("DoResponse error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response body not json: %v\n%s", err, rec.Body.String())
	}
	if m["object"] != "response" || m["status"] != "completed" {
		t.Errorf("response envelope wrong: %v", m)
	}
	output, _ := m["output"].([]any)
	if len(output) != 2 {
		t.Fatalf("output = %v, want 2 items (message + function_call)", output)
	}
	msg, _ := output[0].(map[string]any)
	if msg["type"] != "message" {
		t.Errorf("output[0] = %v, want message", msg)
	}
	fn, _ := output[1].(map[string]any)
	if fn["type"] != "function_call" || fn["name"] != "read" {
		t.Errorf("output[1] = %v, want function_call read", fn)
	}

	// usage 用 OpenAI 语义：input 含缓存
	u, _ := m["usage"].(map[string]any)
	if u["input_tokens"] != float64(14) || u["output_tokens"] != float64(5) {
		t.Errorf("usage = %v, want input=14 output=5", u)
	}
	d, _ := u["input_tokens_details"].(map[string]any)
	if d["cached_tokens"] != float64(4) {
		t.Errorf("input_tokens_details = %v, want cached_tokens=4", d)
	}

	// 计费口径按 OpenAI（relaykit 桥接，P0 已上线行为）：input 含缓存（10+4），
	// CacheIncludedInPrompt=true 由计费侧扣减——金额与 Claude 口径等价
	if usage.PromptTokens != 14 || usage.CompletionTokens != 5 {
		t.Errorf("billing usage = %+v, want prompt=14 completion=5", usage)
	}
	if !usage.CacheIncludedInPrompt {
		t.Errorf("billing usage should set CacheIncludedInPrompt=true: %+v", usage)
	}
}
