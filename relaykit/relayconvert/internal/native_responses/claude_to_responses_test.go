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

// claudeUpstreamMeta 构造 Responses 入站（codex 等）+ Claude 上游渠道的 Meta
func claudeUpstreamMeta(isStream bool) *convmeta.Values {
	return &convmeta.Values{
		OriginModelName:     "glm-5.3",
		UpstreamModelName:   "glm-5.3",
		ChannelMetaAttached: true,
		IsStream:            isStream,
	}
}

// runClaudeToResponsesStream 跑流式转换并收集 StreamEvent 序列
func runClaudeToResponsesStream(t *testing.T, meta convmeta.Meta, upstream string) ([]*relayconvert.StreamEvent, error) {
	t.Helper()
	conv := &ClaudeToResponsesStreamConverter{}
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

// ===== 非流式 =====

// TestClaudeToResponsesResponse_Body Claude 非流式响应转换为 Responses 格式：
// 文本块 → message 项，tool_use 块 → function_call 项，usage 用 OpenAI 语义（input 含缓存）。
func TestClaudeToResponsesResponse_Body(t *testing.T) {
	text := "答案是42"
	claudeResp := &dto.ClaudeResponse{
		ID:    "msg_02",
		Type:  "message",
		Role:  "assistant",
		Model: "glm-5.3",
		Content: []dto.ClaudeContentBlock{
			{Type: "text", Text: &text},
			{Type: "tool_use", ID: "toolu_02", Name: "read", Input: map[string]any{"path": "a.go"}},
		},
		StopReason: "end_turn",
		Usage: &dto.ClaudeUsage{
			InputTokens:          10,
			CacheReadInputTokens: 4,
			OutputTokens:         5,
		},
	}

	conv := &ClaudeToResponsesResponseConverter{}
	res, err := conv.ConvertResponse(context.Background(), claudeUpstreamMeta(false), claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("返回类型应为 map[string]any, got %T", res)
	}

	if m["object"] != "response" || m["status"] != "completed" {
		t.Errorf("response envelope wrong: %v", m)
	}
	if m["id"] != "resp_msg_02" {
		t.Errorf("id = %v, want resp_msg_02", m["id"])
	}
	output, _ := m["output"].([]map[string]any)
	if len(output) != 2 {
		t.Fatalf("output = %v, want 2 items (message + function_call)", output)
	}
	msg := output[0]
	if msg["type"] != "message" {
		t.Errorf("output[0] = %v, want message", msg)
	}
	content := msg["content"].([]map[string]any)
	if len(content) != 1 || content[0]["text"] != "答案是42" {
		t.Errorf("message content 不对: %+v", content)
	}
	fn := output[1]
	if fn["type"] != "function_call" || fn["name"] != "read" {
		t.Errorf("output[1] = %v, want function_call read", fn)
	}
	if fn["id"] != "toolu_02" || fn["call_id"] != "toolu_02" {
		t.Errorf("function_call id 应搬运 Claude tool_use.id: %+v", fn)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(fn["arguments"].(string)), &args); err != nil {
		t.Fatalf("arguments 应为 JSON 字符串: %v", err)
	}
	if args["path"] != "a.go" {
		t.Errorf("参数丢失: %+v", args)
	}

	// usage 用 OpenAI 语义：input 含缓存
	u, _ := m["usage"].(map[string]any)
	if u["input_tokens"].(int) != 14 || u["output_tokens"].(int) != 5 {
		t.Errorf("usage = %v, want input=14 output=5", u)
	}
	d, _ := u["input_tokens_details"].(map[string]any)
	if d["cached_tokens"].(int) != 4 {
		t.Errorf("input_tokens_details = %v, want cached_tokens=4", d)
	}
}

// TestClaudeToResponsesResponse_ThinkingSkipped 思考内容无 Responses 非流式对应物，跳过
// TestClaudeToResponsesResponse_ThinkingBecomesReasoningItem Claude 的 thinking 块
// 转换为 Responses 的 reasoning 输出项（排在 message 之前），而不是被丢弃或混进正文。
//
// 本测试此前名为 ThinkingSkipped、断言 output 只有 1 项——把「思考内容静默丢失」
// 固化成了期望。Responses 非流式本就有 reasoning 输出项形态（流式侧的
// response.reasoning_summary_text.delta 只是它的增量表达），跳过是能力失恒。
func TestClaudeToResponsesResponse_ThinkingBecomesReasoningItem(t *testing.T) {
	thinking := "想一想"
	text := "你好"
	claudeResp := &dto.ClaudeResponse{
		ID: "msg_1",
		Content: []dto.ClaudeContentBlock{
			{Type: "thinking", Thinking: &thinking},
			{Type: "redacted_thinking"},
			{Type: "text", Text: &text},
		},
		StopReason: "end_turn",
	}

	conv := &ClaudeToResponsesResponseConverter{}
	res, err := conv.ConvertResponse(context.Background(), claudeUpstreamMeta(false), claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	m := res.(map[string]any)
	output := m["output"].([]map[string]any)
	if len(output) != 2 {
		t.Fatalf("output 项数 = %d, want 2（reasoning + message）: %+v", len(output), output)
	}

	// reasoning 项在最前，与 Responses API 的真实输出顺序一致
	if output[0]["type"] != "reasoning" {
		t.Fatalf("首项应为 reasoning, got %v", output[0]["type"])
	}
	summary := output[0]["summary"].([]map[string]any)
	if len(summary) != 1 || summary[0]["type"] != "summary_text" || summary[0]["text"] != "想一想" {
		t.Errorf("reasoning.summary 形态不符: %+v", summary)
	}

	// 正文不得混入思考内容
	if output[1]["type"] != "message" {
		t.Fatalf("次项应为 message, got %v", output[1]["type"])
	}
	content := output[1]["content"].([]map[string]any)
	if content[0]["text"] != "你好" {
		t.Errorf("思考内容不得混进正文: %+v", content)
	}
}

// ===== 流式 =====

// TestClaudeToResponsesStream_TextAndToolCall
// Claude SSE（文本 + 工具调用）完整转换为 Responses 事件流：
// 事件序列、工具参数聚合、completed 的 usage 映射（OpenAI 语义）。
func TestClaudeToResponsesStream_TextAndToolCall(t *testing.T) {
	upstream := strings.Join([]string{
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

	events, err := runClaudeToResponsesStream(t, claudeUpstreamMeta(true), upstream)
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

	if responsesDataOf(t, events[3])["delta"] != "你好" {
		t.Errorf("text delta 不对: %+v", responsesDataOf(t, events[3]))
	}
	if responsesDataOf(t, events[8])["delta"] != `{"cmd":"ls"}` {
		t.Errorf("tool arguments delta 不对: %+v", responsesDataOf(t, events[8]))
	}
	toolItem := responsesDataOf(t, events[7])["item"].(map[string]any)
	if toolItem["name"] != "shell" || toolItem["id"] != "toolu_01" {
		t.Errorf("function_call 项不对: %+v", toolItem)
	}

	// completed 的 usage 用 OpenAI 语义：input 含缓存（10+4），cached_tokens 为子集
	completed := responsesDataOf(t, events[11])["response"].(map[string]any)
	u := completed["usage"].(map[string]any)
	if u["input_tokens"].(int) != 14 {
		t.Errorf("completed usage input_tokens = %v, want 14 (input+cache_read)", u["input_tokens"])
	}
	if u["input_tokens_details"].(map[string]any)["cached_tokens"].(int) != 4 {
		t.Errorf("completed usage cached_tokens = %v, want 4", u["input_tokens_details"])
	}
	if u["output_tokens"].(int) != 7 {
		t.Errorf("completed usage output_tokens = %v, want 7", u["output_tokens"])
	}

	// completed 的 output 数组：message + function_call
	out := completed["output"].([]map[string]any)
	if len(out) != 2 || out[0]["type"] != "message" || out[1]["type"] != "function_call" {
		t.Errorf("completed output 不对: %+v", out)
	}

	// 计费用量随 completed 帧携带，按 Claude 口径（input 不含缓存，cache 明细独立）
	if events[11].Usage == nil {
		t.Fatal("completed 帧必须携带计费用量")
	}
	if events[11].Usage.PromptTokens != 10 || events[11].Usage.CompletionTokens != 7 {
		t.Errorf("billing usage = %+v, want prompt=10 completion=7", events[11].Usage)
	}
	if events[11].Usage.PromptTokensDetails == nil || events[11].Usage.PromptTokensDetails.CachedTokens != 4 {
		t.Errorf("billing usage cache detail = %+v, want cached_tokens=4", events[11].Usage.PromptTokensDetails)
	}
}

// TestClaudeToResponsesStream_UpstreamErrorBeforeEvents
// 首个事件即 error：返回错误且不合成假成功的 response.completed。
func TestClaudeToResponsesStream_UpstreamErrorBeforeEvents(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
		``,
	}, "\n")

	events, err := runClaudeToResponsesStream(t, claudeUpstreamMeta(true), upstream)
	if err == nil {
		t.Fatal("upstream error event should return an error, got nil")
	}
	if !strings.Contains(err.Error(), "overloaded_error") {
		t.Errorf("错误信息应包含上游错误详情: %v", err)
	}
	for _, ev := range events {
		if ev.Event == "response.completed" {
			t.Errorf("上游错误不应合成 response.completed: %v", responsesEventTypes(t, events))
		}
	}
}

// TestClaudeToResponsesStream_UnfinishedStreamStillCompletes 上游未发 message_stop 即断流：
// 仍合成 completed，避免客户端挂起。
func TestClaudeToResponsesStream_UnfinishedStreamStillCompletes(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_01","model":"glm-5.3","usage":{"input_tokens":10,"output_tokens":1}}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"半截"}}`,
		``,
	}, "\n")

	events, err := runClaudeToResponsesStream(t, claudeUpstreamMeta(true), upstream)
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}
	got := responsesEventTypes(t, events)
	if len(got) == 0 || got[len(got)-1] != "response.completed" {
		t.Fatalf("必须以 response.completed 收尾, got %v", got)
	}
}

// TestClaudeToResponsesStream_ThinkingAsReasoningSummary 思考增量以 reasoning summary 事件透出
func TestClaudeToResponsesStream_ThinkingAsReasoningSummary(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_01","model":"glm-5.3","usage":{"input_tokens":5,"output_tokens":0}}}`,
		``,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"推理中"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig-1"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	events, err := runClaudeToResponsesStream(t, claudeUpstreamMeta(true), upstream)
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}
	got := responsesEventTypes(t, events)
	want := []string{
		"response.created",
		"response.output_item.added",
		"response.content_part.added",
		"response.reasoning_summary_text.delta", // signature_delta 无对应物被忽略
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.completed",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("事件序列不符\ngot:  %v\nwant: %v", got, want)
	}
	if responsesDataOf(t, events[3])["delta"] != "推理中" {
		t.Errorf("reasoning delta 不对: %+v", responsesDataOf(t, events[3]))
	}
}
