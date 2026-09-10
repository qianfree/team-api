package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

// responsesInboundAdaptor 构造 Responses 入站 + Gemini 上游的已 Init 适配器
func responsesInboundAdaptor(stream bool) (*Adaptor, *common.RelayInfo) {
	info := &common.RelayInfo{
		RelayMode:       int(constant.RelayModeResponses),
		IsStream:        stream,
		RequestID:       "req123",
		InboundFormat:   constant.RelayFormatResponses,
		ClientFormat:    constant.RelayFormatResponses,
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

// parseResponsesSSE 解析 Responses 格式 SSE（event + data 行）为有序事件列表。
// 复用 claude_bridge_test.go 的 sseEvent / parseClaudeSSE —— 两者帧格式相同。
func parseResponsesSSE(t *testing.T, raw string) []sseEvent {
	t.Helper()
	return parseClaudeSSE(t, raw)
}

// ===== 非流式 =====

// TestBuildResponsesOutputFromGemini_TextAndTool 文本合并为居首的 message 项，
// functionCall 各成一个 function_call 项；思考内容在非流式响应中跳过。
func TestBuildResponsesOutputFromGemini_TextAndTool(t *testing.T) {
	thought := true
	_, info := responsesInboundAdaptor(false)
	geminiResp := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{{
			Content: &dto.GeminiContent{
				Role: "model",
				Parts: []dto.GeminiPart{
					{Text: "思考中", Thought: &thought},
					{Text: "你好"},
					{Text: "世界"},
					{FunctionCall: &dto.GeminiFunctionCall{
						FunctionName: "get_weather",
						Arguments:    map[string]any{"city": "北京"},
					}},
				},
			},
			FinishReason: "STOP",
		}},
	}

	output := buildResponsesOutputFromGemini(geminiResp, info)

	if len(output) != 2 {
		t.Fatalf("output 项数 = %d, want 2（message + function_call）: %+v", len(output), output)
	}

	msg := output[0]
	if msg["type"] != "message" || msg["status"] != "completed" || msg["role"] != "assistant" {
		t.Errorf("首项应为 completed 的 assistant message: %+v", msg)
	}
	content := msg["content"].([]map[string]any)
	if len(content) != 1 || content[0]["type"] != "output_text" {
		t.Fatalf("message content 结构不对: %+v", content)
	}
	// 思考内容不得混进正文
	if got := content[0]["text"]; got != "你好世界" {
		t.Errorf("正文 = %q, want %q（思考内容应被跳过）", got, "你好世界")
	}

	fc := output[1]
	if fc["type"] != "function_call" || fc["name"] != "get_weather" {
		t.Errorf("次项应为 function_call: %+v", fc)
	}
	if fc["id"] != fc["call_id"] {
		t.Errorf("id 与 call_id 应一致: %+v", fc)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(fc["arguments"].(string)), &args); err != nil {
		t.Fatalf("arguments 应为 JSON 字符串: %v", err)
	}
	if args["city"] != "北京" {
		t.Errorf("参数丢失: %+v", args)
	}
}

// TestGeminiResponsesCallID_PrefersUpstreamID Gemini 的 functionCall.id 可选，
// 有则直接用，无则合成（Responses 的 function_call 必须带 call_id）。
func TestGeminiResponsesCallID_PrefersUpstreamID(t *testing.T) {
	_, info := responsesInboundAdaptor(false)

	withID := geminiResponsesCallID(&dto.GeminiFunctionCall{ID: "up_1", FunctionName: "f"}, info, 0)
	if withID != "up_1" {
		t.Errorf("有上游 id 时应直接使用, got %q", withID)
	}

	a := geminiResponsesCallID(&dto.GeminiFunctionCall{FunctionName: "f"}, info, 0)
	b := geminiResponsesCallID(&dto.GeminiFunctionCall{FunctionName: "f"}, info, 1)
	if a == "" || a == b {
		t.Errorf("合成 ID 必须非空且互不相同: %q / %q", a, b)
	}
	if !strings.HasPrefix(a, "call_") {
		t.Errorf("合成 ID 应为 call_ 前缀: %q", a)
	}
}

// TestHandleNonStreamToResponses_WritesResponsesBody 正常路径：响应体为 Responses 对象，
// 用量按 OpenAI 语义（input 含缓存、output 含思考、reasoning_tokens 单列）。
func TestHandleNonStreamToResponses_WritesResponsesBody(t *testing.T) {
	body := `{"candidates":[{"content":{"role":"model","parts":[{"text":"你好"}]},"finishReason":"STOP"}],` +
		`"modelVersion":"gemini-3-pro","responseId":"gr_1",` +
		`"usageMetadata":{"promptTokenCount":50,"cachedContentTokenCount":10,"candidatesTokenCount":8,"thoughtsTokenCount":3,"totalTokenCount":61}}`

	a, info := responsesInboundAdaptor(false)
	rec := httptest.NewRecorder()

	usage, err := a.handleNonStreamToResponses(context.Background(), jsonResponse(http.StatusOK, body), info, rec)
	if err != nil {
		t.Fatalf("handleNonStreamToResponses error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应体不是合法 JSON: %v\nbody=%s", err, rec.Body.String())
	}
	if got["object"] != "response" || got["status"] != "completed" {
		t.Errorf("Responses 骨架不对: object=%v status=%v", got["object"], got["status"])
	}
	if id, _ := got["id"].(string); id != "resp_gr_1" {
		t.Errorf("id = %q, want resp_gr_1（应优先用上游 responseId）", id)
	}
	if got["completed_at"] == nil {
		t.Error("completed 状态必须带 completed_at")
	}

	u := got["usage"].(map[string]any)
	if u["input_tokens"].(float64) != 50 {
		t.Errorf("input_tokens = %v, want 50（OpenAI 语义含缓存）", u["input_tokens"])
	}
	if u["output_tokens"].(float64) != 11 {
		t.Errorf("output_tokens = %v, want 11（candidates+thoughts）", u["output_tokens"])
	}
	if rt := u["output_tokens_details"].(map[string]any)["reasoning_tokens"].(float64); rt != 3 {
		t.Errorf("reasoning_tokens = %v, want 3", rt)
	}
	if ct := u["input_tokens_details"].(map[string]any)["cached_tokens"].(float64); ct != 10 {
		t.Errorf("cached_tokens = %v, want 10", ct)
	}

	// 计费口径与客户端可见口径在本方向一致
	if usage.PromptTokens != 50 || usage.CompletionTokens != 11 || !usage.CacheIncludedInPrompt {
		t.Errorf("计费用量不对: %+v", usage)
	}
}

// TestHandleNonStreamToResponses_UpstreamErrorNotWritten 上游错误不在本层写响应
func TestHandleNonStreamToResponses_UpstreamErrorNotWritten(t *testing.T) {
	a, info := responsesInboundAdaptor(false)
	rec := httptest.NewRecorder()
	body := `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","message":"quota"}}`

	_, err := a.handleNonStreamToResponses(context.Background(), jsonResponse(http.StatusTooManyRequests, body), info, rec)

	if err == nil {
		t.Fatal("上游 429 应返回错误")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("本层不应写响应体, got %q", rec.Body.String())
	}
}

// TestHandleNonStreamToResponses_SafetyBlock 安全过滤命中时返回请求类错误且不写响应体
func TestHandleNonStreamToResponses_SafetyBlock(t *testing.T) {
	a, info := responsesInboundAdaptor(false)
	rec := httptest.NewRecorder()

	_, err := a.handleNonStreamToResponses(context.Background(),
		jsonResponse(http.StatusOK, `{"promptFeedback":{"blockReason":"SAFETY"}}`), info, rec)

	if err == nil {
		t.Fatal("安全过滤应返回错误")
	}
	assertContentBlockedNotUpstream(t, err)
	if rec.Body.Len() != 0 {
		t.Errorf("不应写响应体, got %q", rec.Body.String())
	}
}

// ===== 流式 =====

// TestHandleStreamToResponses_EventSequence 完整事件序列：
// created → output_item.added → content_part.added → 文本/思考增量 → 三个 done → completed
func TestHandleStreamToResponses_EventSequence(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"想一下","thought":true}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"你好"}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"世界"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":50,"cachedContentTokenCount":10,"candidatesTokenCount":8,"thoughtsTokenCount":3,"totalTokenCount":61}}`,
		``,
		``,
	}, "\n")

	a, info := responsesInboundAdaptor(true)
	rec := httptest.NewRecorder()

	usage, err := a.handleStreamToResponses(context.Background(), sseResponse(strings.NewReader(upstream)), info, rec)
	if err != nil {
		t.Fatalf("handleStreamToResponses error: %v", err)
	}

	events := parseResponsesSSE(t, rec.Body.String())
	got := eventTypes(events)
	want := []string{
		"response.created",
		"response.output_item.added",
		"response.content_part.added",
		"response.reasoning_summary_text.delta",
		"response.output_text.delta",
		"response.output_text.delta",
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.completed",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("事件序列不符\ngot:  %v\nwant: %v", got, want)
	}

	// 思考增量走 reasoning summary，不得混进正文
	if events[3].Data["delta"] != "想一下" {
		t.Errorf("reasoning delta 不对: %+v", events[3].Data)
	}
	if events[6].Data["text"] != "你好世界" {
		t.Errorf("output_text.done 文本 = %v, want 你好世界", events[6].Data["text"])
	}

	completed := events[9].Data["response"].(map[string]any)
	if completed["status"] != "completed" {
		t.Errorf("最终状态 = %v", completed["status"])
	}
	out := completed["output"].([]any)
	if len(out) != 1 || out[0].(map[string]any)["type"] != "message" {
		t.Fatalf("completed.output 应含单个 message 项: %+v", out)
	}
	u := completed["usage"].(map[string]any)
	if u["input_tokens"].(float64) != 50 || u["output_tokens"].(float64) != 11 {
		t.Errorf("completed usage 不对: %+v", u)
	}

	if usage.PromptTokens != 50 || usage.CompletionTokens != 11 {
		t.Errorf("计费用量 = %d/%d, want 50/11", usage.PromptTokens, usage.CompletionTokens)
	}
}

// TestHandleStreamToResponses_ToolCall 工具调用前必须先关闭文本 content part，
// 且 function_call 项的 output_index 与文本项不冲突。
func TestHandleStreamToResponses_ToolCall(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"查一下"}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"city":"北京"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":4,"totalTokenCount":14}}`,
		``,
		``,
	}, "\n")

	a, info := responsesInboundAdaptor(true)
	rec := httptest.NewRecorder()

	_, err := a.handleStreamToResponses(context.Background(), sseResponse(strings.NewReader(upstream)), info, rec)
	if err != nil {
		t.Fatalf("handleStreamToResponses error: %v", err)
	}

	events := parseResponsesSSE(t, rec.Body.String())
	got := eventTypes(events)
	want := []string{
		"response.created",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
		// 工具调用前先收掉文本 part
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.output_item.added",
		"response.function_call_arguments.delta",
		"response.function_call_arguments.done",
		"response.output_item.done",
		"response.completed",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("事件序列不符\ngot:  %v\nwant: %v", got, want)
	}

	// 文本项 output_index=0，工具项 output_index=1
	if idx := events[6].Data["output_index"].(float64); idx != 0 {
		t.Errorf("文本项 output_index = %v, want 0", idx)
	}
	if idx := events[7].Data["output_index"].(float64); idx != 1 {
		t.Errorf("工具项 output_index = %v, want 1", idx)
	}

	toolItem := events[7].Data["item"].(map[string]any)
	if toolItem["type"] != "function_call" || toolItem["name"] != "get_weather" {
		t.Errorf("function_call 项不对: %+v", toolItem)
	}

	completed := events[11].Data["response"].(map[string]any)
	out := completed["output"].([]any)
	if len(out) != 2 {
		t.Fatalf("completed.output 应含 message + function_call: %+v", out)
	}
	if out[0].(map[string]any)["type"] != "message" || out[1].(map[string]any)["type"] != "function_call" {
		t.Errorf("output 顺序不对: %+v", out)
	}
}

// TestHandleStreamToResponses_EmptyUpstreamStillCompletes 上游无有效 chunk 时仍须合成
// created/completed，否则 Responses 客户端一直等 response.completed。
func TestHandleStreamToResponses_EmptyUpstreamStillCompletes(t *testing.T) {
	a, info := responsesInboundAdaptor(true)
	rec := httptest.NewRecorder()

	_, err := a.handleStreamToResponses(context.Background(), sseResponse(strings.NewReader("\n\n")), info, rec)
	if err != nil {
		t.Fatalf("handleStreamToResponses error: %v", err)
	}

	got := eventTypes(parseResponsesSSE(t, rec.Body.String()))
	if len(got) == 0 || got[0] != "response.created" || got[len(got)-1] != "response.completed" {
		t.Fatalf("必须以 created 开场、completed 收尾, got %v", got)
	}
}

// TestHandleStreamToResponses_SafetyBlockCompletes 安全过滤命中时 SSE 头已发出，
// 必须补齐 completed（否则客户端挂起）并向上返回请求类错误（同 handleStreamToClaude 口径）。
func TestHandleStreamToResponses_SafetyBlockCompletes(t *testing.T) {
	a, info := responsesInboundAdaptor(true)
	rec := httptest.NewRecorder()

	_, err := a.handleStreamToResponses(context.Background(),
		sseResponse(strings.NewReader("data: {\"promptFeedback\":{\"blockReason\":\"SAFETY\"}}\n\n")), info, rec)
	if err == nil {
		t.Fatal("安全过滤应返回错误")
	}
	assertContentBlockedError(t, err)
	if got := info.StreamStatus.GetEndReason(); got != common.StreamEndReasonError {
		t.Errorf("StreamStatus end reason = %v, want %v", got, common.StreamEndReasonError)
	}

	got := eventTypes(parseResponsesSSE(t, rec.Body.String()))
	if len(got) == 0 || got[len(got)-1] != "response.completed" {
		t.Errorf("必须以 response.completed 收尾, got %v", got)
	}
}

// TestHandleStreamToResponses_UpstreamErrorNotWritten 流式上游错误不在本层写响应
func TestHandleStreamToResponses_UpstreamErrorNotWritten(t *testing.T) {
	a, info := responsesInboundAdaptor(true)
	rec := httptest.NewRecorder()
	resp := jsonResponse(http.StatusServiceUnavailable, `{"error":{"code":503,"status":"UNAVAILABLE","message":"down"}}`)

	_, err := a.handleStreamToResponses(context.Background(), resp, info, rec)

	if err == nil {
		t.Fatal("上游 503 应返回错误")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("本层不应写响应体, got %q", rec.Body.String())
	}
}
