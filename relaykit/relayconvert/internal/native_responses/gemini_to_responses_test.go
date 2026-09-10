package native_responses

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// responsesInboundMeta 构造 Responses 入站 + Gemini 上游的 Meta
func responsesInboundMeta(isStream bool) *convmeta.Values {
	return &convmeta.Values{
		OriginModelName:     "gemini-3-pro",
		UpstreamModelName:   "gemini-3-pro",
		ChannelMetaAttached: true,
		IsStream:            isStream,
	}
}

// runGeminiToResponsesStream 跑流式转换并收集 StreamEvent 序列
func runGeminiToResponsesStream(t *testing.T, meta convmeta.Meta, upstream string) ([]*relayconvert.StreamEvent, error) {
	t.Helper()
	conv := &GeminiToResponsesStreamConverter{}
	var events []*relayconvert.StreamEvent
	err := conv.ConvertStreamResponse(context.Background(), meta, strings.NewReader(upstream), func(chunk any) error {
		ev, ok := chunk.(*relayconvert.StreamEvent)
		if !ok {
			t.Fatalf("chunk 类型应为 *relayconvert.StreamEvent, got %T", chunk)
		}
		events = append(events, ev)
		return nil
	})
	return events, err
}

// responsesEventTypes 提取事件名序列（校验帧形态：Responses 客户端为事件帧）
func responsesEventTypes(t *testing.T, events []*relayconvert.StreamEvent) []string {
	t.Helper()
	names := make([]string, 0, len(events))
	for _, ev := range events {
		if ev.Event == "" {
			t.Fatalf("Responses 客户端应为事件帧（Event 非空）: %+v", ev)
		}
		names = append(names, ev.Event)
	}
	return names
}

// responsesDataOf 取事件负载并断言为 map
func responsesDataOf(t *testing.T, ev *relayconvert.StreamEvent) map[string]any {
	t.Helper()
	data, ok := ev.Data.(map[string]any)
	if !ok {
		t.Fatalf("data 类型应为 map[string]any, got %T", ev.Data)
	}
	return data
}

// ===== 非流式 =====

// TestBuildResponsesOutputFromGemini_TextAndTool 文本合并为居首的 message 项，
// functionCall 各成一个 function_call 项；思考内容在非流式响应中跳过。
func TestBuildResponsesOutputFromGemini_TextAndTool(t *testing.T) {
	thought := true
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

	output := buildResponsesOutputFromGemini(geminiResp, "req123")

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
	withID := geminiResponsesCallID(&dto.GeminiFunctionCall{ID: "up_1", FunctionName: "f"}, "base", 0)
	if withID != "up_1" {
		t.Errorf("有上游 id 时应直接使用, got %q", withID)
	}

	a := geminiResponsesCallID(&dto.GeminiFunctionCall{FunctionName: "f"}, "base", 0)
	b := geminiResponsesCallID(&dto.GeminiFunctionCall{FunctionName: "f"}, "base", 1)
	if a == "" || a == b {
		t.Errorf("合成 ID 必须非空且互不相同: %q / %q", a, b)
	}
	if !strings.HasPrefix(a, "call_") {
		t.Errorf("合成 ID 应为 call_ 前缀: %q", a)
	}
}

// TestGeminiToResponsesResponse_Body 正常路径：转换结果为 Responses 对象，
// 用量按 OpenAI 语义（input 含缓存、output 含思考、reasoning_tokens 单列）。
func TestGeminiToResponsesResponse_Body(t *testing.T) {
	geminiResp := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{{
			Content:      &dto.GeminiContent{Role: "model", Parts: []dto.GeminiPart{{Text: "你好"}}},
			FinishReason: "STOP",
		}},
		ModelName:  "gemini-3-pro",
		ResponseID: "gr_1",
		UsageMetadata: &dto.GeminiUsageMetadata{
			PromptTokenCount:        50,
			CachedContentTokenCount: 10,
			CandidatesTokenCount:    8,
			ThoughtsTokenCount:      3,
			TotalTokenCount:         61,
		},
	}

	conv := &GeminiToResponsesResponseConverter{}
	res, err := conv.ConvertResponse(context.Background(), responsesInboundMeta(false), geminiResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	got, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("返回类型应为 map[string]any, got %T", res)
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
	if got["model"] != "gemini-3-pro" {
		t.Errorf("model = %v, want gemini-3-pro", got["model"])
	}

	u := got["usage"].(map[string]any)
	if u["input_tokens"].(int) != 50 {
		t.Errorf("input_tokens = %v, want 50（OpenAI 语义含缓存）", u["input_tokens"])
	}
	if u["output_tokens"].(int) != 11 {
		t.Errorf("output_tokens = %v, want 11（candidates+thoughts）", u["output_tokens"])
	}
	if rt := u["output_tokens_details"].(map[string]any)["reasoning_tokens"].(int); rt != 3 {
		t.Errorf("reasoning_tokens = %v, want 3", rt)
	}
	if ct := u["input_tokens_details"].(map[string]any)["cached_tokens"].(int); ct != 10 {
		t.Errorf("cached_tokens = %v, want 10", ct)
	}
}

// TestGeminiToResponsesResponse_SafetyBlock 安全过滤命中时返回错误而非合成响应
func TestGeminiToResponsesResponse_SafetyBlock(t *testing.T) {
	conv := &GeminiToResponsesResponseConverter{}
	_, err := conv.ConvertResponse(context.Background(), responsesInboundMeta(false), &dto.GeminiChatResponse{
		PromptFeedback: &dto.GeminiPromptFeedback{BlockReason: "SAFETY"},
	})
	if err == nil {
		t.Fatal("安全过滤应返回错误")
	}
	if !strings.Contains(err.Error(), "SAFETY") {
		t.Errorf("错误信息应包含 blockReason: %v", err)
	}
}

// ===== 流式 =====

// TestGeminiToResponsesStream_EventSequence 完整事件序列：
// created → output_item.added → content_part.added → 文本/思考增量 → 三个 done → completed
func TestGeminiToResponsesStream_EventSequence(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"想一下","thought":true}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"你好"}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"世界"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":50,"cachedContentTokenCount":10,"candidatesTokenCount":8,"thoughtsTokenCount":3,"totalTokenCount":61}}`,
		``,
		``,
	}, "\n")

	events, err := runGeminiToResponsesStream(t, responsesInboundMeta(true), upstream)
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	got := responsesEventTypes(t, events)
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
	if responsesDataOf(t, events[3])["delta"] != "想一下" {
		t.Errorf("reasoning delta 不对: %+v", responsesDataOf(t, events[3]))
	}
	if responsesDataOf(t, events[6])["text"] != "你好世界" {
		t.Errorf("output_text.done 文本 = %v, want 你好世界", responsesDataOf(t, events[6])["text"])
	}

	completed := responsesDataOf(t, events[9])["response"].(map[string]any)
	if completed["status"] != "completed" {
		t.Errorf("最终状态 = %v", completed["status"])
	}
	out := completed["output"].([]map[string]any)
	if len(out) != 1 || out[0]["type"] != "message" {
		t.Fatalf("completed.output 应含单个 message 项: %+v", out)
	}
	u := completed["usage"].(map[string]any)
	if u["input_tokens"].(int) != 50 || u["output_tokens"].(int) != 11 {
		t.Errorf("completed usage 不对: %+v", u)
	}

	// 计费用量随 completed 帧携带（本方向与客户端可见口径一致）
	if events[9].Usage == nil {
		t.Fatal("completed 帧必须携带计费用量")
	}
	if events[9].Usage.PromptTokens != 50 || events[9].Usage.CompletionTokens != 11 {
		t.Errorf("计费用量 = %d/%d, want 50/11", events[9].Usage.PromptTokens, events[9].Usage.CompletionTokens)
	}
	if events[9].Usage.PromptTokensDetails == nil || events[9].Usage.PromptTokensDetails.CachedTokens != 10 {
		t.Errorf("计费缓存明细不对: %+v", events[9].Usage.PromptTokensDetails)
	}
}

// TestGeminiToResponsesStream_ToolCall 工具调用前必须先关闭文本 content part，
// 且 function_call 项的 output_index 与文本项不冲突。
func TestGeminiToResponsesStream_ToolCall(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"查一下"}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"city":"北京"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":4,"totalTokenCount":14}}`,
		``,
		``,
	}, "\n")

	events, err := runGeminiToResponsesStream(t, responsesInboundMeta(true), upstream)
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	got := responsesEventTypes(t, events)
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
	if idx := responsesDataOf(t, events[6])["output_index"].(int); idx != 0 {
		t.Errorf("文本项 output_index = %v, want 0", idx)
	}
	if idx := responsesDataOf(t, events[7])["output_index"].(int); idx != 1 {
		t.Errorf("工具项 output_index = %v, want 1", idx)
	}

	toolItem := responsesDataOf(t, events[7])["item"].(map[string]any)
	if toolItem["type"] != "function_call" || toolItem["name"] != "get_weather" {
		t.Errorf("function_call 项不对: %+v", toolItem)
	}

	completed := responsesDataOf(t, events[11])["response"].(map[string]any)
	out := completed["output"].([]map[string]any)
	if len(out) != 2 {
		t.Fatalf("completed.output 应含 message + function_call: %+v", out)
	}
	if out[0]["type"] != "message" || out[1]["type"] != "function_call" {
		t.Errorf("output 顺序不对: %+v", out)
	}
}

// TestGeminiToResponsesStream_EmptyUpstreamStillCompletes 上游无有效 chunk 时仍须合成
// created/completed，否则 Responses 客户端一直等 response.completed。
func TestGeminiToResponsesStream_EmptyUpstreamStillCompletes(t *testing.T) {
	events, err := runGeminiToResponsesStream(t, responsesInboundMeta(true), "\n\n")
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	got := responsesEventTypes(t, events)
	if len(got) == 0 || got[0] != "response.created" || got[len(got)-1] != "response.completed" {
		t.Fatalf("必须以 created 开场、completed 收尾, got %v", got)
	}
}

// TestGeminiToResponsesStream_SafetyBlockCompletes 安全过滤命中时事件流已开场，
// 必须补齐 completed 再返回错误。
func TestGeminiToResponsesStream_SafetyBlockCompletes(t *testing.T) {
	events, err := runGeminiToResponsesStream(t, responsesInboundMeta(true),
		"data: {\"promptFeedback\":{\"blockReason\":\"SAFETY\"}}\n\n")
	if err == nil {
		t.Fatal("安全过滤应返回错误")
	}

	got := responsesEventTypes(t, events)
	if len(got) == 0 || got[len(got)-1] != "response.completed" {
		t.Errorf("必须以 response.completed 收尾, got %v", got)
	}
}

// stashMeta 测试用 Meta 实现：嵌入 convmeta.Values 并实现 ResponsesStash 能力接口
// （对应宿主 RelayInfo 的 info.ResponsesRequest 字段）。
type stashMeta struct {
	*convmeta.Values
	stashed *dto.OpenAIResponsesRequest
}

func (m *stashMeta) StashResponsesRequest(req *dto.OpenAIResponsesRequest) { m.stashed = req }
func (m *stashMeta) StashedResponsesRequest() *dto.OpenAIResponsesRequest  { return m.stashed }

// TestResponsesObjectEchoesStashedRequest 合成响应应 echo 宿主 stash 的请求参数
// （temperature / top_p / max_output_tokens / instructions），未 stash 时回退默认值。
func TestResponsesObjectEchoesStashedRequest(t *testing.T) {
	temp := 0.3
	topP := 0.9
	maxOut := uint(256)
	meta := &stashMeta{Values: responsesInboundMeta(false)}
	meta.StashResponsesRequest(&dto.OpenAIResponsesRequest{
		Temperature:     &temp,
		TopP:            &topP,
		MaxOutputTokens: &maxOut,
		Instructions:    json.RawMessage(`"be brief"`),
	})

	conv := &GeminiToResponsesResponseConverter{}
	res, err := conv.ConvertResponse(context.Background(), meta, &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{{
			Content:      &dto.GeminiContent{Role: "model", Parts: []dto.GeminiPart{{Text: "hi"}}},
			FinishReason: "STOP",
		}},
	})
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	got := res.(map[string]any)
	if got["temperature"] != 0.3 || got["top_p"] != 0.9 {
		t.Errorf("temperature/top_p 未 echo: %v / %v", got["temperature"], got["top_p"])
	}
	if mo, ok := got["max_output_tokens"].(*int); !ok || *mo != 256 {
		t.Errorf("max_output_tokens 未 echo: %v", got["max_output_tokens"])
	}
	if string(got["instructions"].(json.RawMessage)) != `"be brief"` {
		t.Errorf("instructions 未 echo: %v", got["instructions"])
	}

	// 未 stash：回退默认值
	res2, err := conv.ConvertResponse(context.Background(), responsesInboundMeta(false), &dto.GeminiChatResponse{})
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	got2 := res2.(map[string]any)
	if got2["temperature"] != 1.0 || got2["top_p"] != 1.0 {
		t.Errorf("默认 temperature/top_p 不对: %v / %v", got2["temperature"], got2["top_p"])
	}
}
