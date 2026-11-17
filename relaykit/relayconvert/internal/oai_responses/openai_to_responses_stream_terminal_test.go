package oai_responses

import (
	"errors"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// 终止态与收尾语义：chat 上游如何结束，决定 Codex 客户端眼中回答是「完整 / 被截断 / 失败」。
// 这里任何一处把异常包装成 completed，客户端表现都是"回答到一半正常停止"。

// chatChunk 构造一行 chat SSE data（choices 固定单 choice）。
func chatChunk(deltaJSON string, finishReason string) string {
	fr := "null"
	if finishReason != "" {
		fr = `"` + finishReason + `"`
	}
	return `data: {"id":"abc","object":"chat.completion.chunk","created":111,"model":"gpt-4o","choices":[{"index":0,"delta":` + deltaJSON + `,"finish_reason":` + fr + `}]}`
}

// responseObj 取终止事件负载里的 response 对象。
func responseObj(t *testing.T, ev *relayconvert.StreamEvent) map[string]any {
	t.Helper()
	resp, ok := eventData(t, ev)["response"].(map[string]any)
	if !ok {
		t.Fatalf("event %s has no response object", ev.Event)
	}
	return resp
}

// TestOpenAIToResponsesStream_LengthIncomplete 上游 finish_reason=length（被 max_tokens 截断）：
// 终止事件必须是 response.incomplete（status=incomplete + incomplete_details.reason=
// max_output_tokens），message 项 status=incomplete。此前一律 response.completed，
// codex 把被截断的回答当完整答案、不提示续写。
func TestOpenAIToResponsesStream_LengthIncomplete(t *testing.T) {
	sse := strings.Join([]string{
		chatChunk(`{"role":"assistant","content":"The answer is"}`, ""),
		``,
		chatChunk(`{}`, "length"),
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	events, err := runChatToResponsesStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	last := events[len(events)-1]
	if last.Event != "response.incomplete" {
		t.Fatalf("terminal event = %s, want response.incomplete (events: %v)", last.Event, eventNames(events))
	}
	if eventData(t, last)["type"] != "response.incomplete" {
		t.Errorf("payload type = %v, want response.incomplete", eventData(t, last)["type"])
	}
	resp := responseObj(t, last)
	if resp["status"] != "incomplete" {
		t.Errorf("status = %v, want incomplete", resp["status"])
	}
	details, _ := resp["incomplete_details"].(map[string]any)
	if details == nil || details["reason"] != "max_output_tokens" {
		t.Errorf("incomplete_details = %v, want reason=max_output_tokens", resp["incomplete_details"])
	}
	// output_item.done 与终止帧 output 里的 message 项都应标 incomplete，文本仍完整携带
	var sawItemDone bool
	for _, ev := range events {
		if ev.Event != "response.output_item.done" {
			continue
		}
		item, _ := eventData(t, ev)["item"].(map[string]any)
		if item["type"] == "message" {
			sawItemDone = true
			if item["status"] != "incomplete" {
				t.Errorf("output_item.done message status = %v, want incomplete", item["status"])
			}
		}
	}
	if !sawItemDone {
		t.Error("message output_item.done not emitted")
	}
	output, _ := resp["output"].([]map[string]any)
	if len(output) != 1 || output[0]["status"] != "incomplete" {
		t.Errorf("final output = %v, want single message with status=incomplete", resp["output"])
	}
	// usage 仍挂在终止事件上供宿主计费（文本估算 13 字节 / 4 = 3）
	if last.Usage == nil || last.Usage.CompletionTokens != 3 {
		t.Errorf("terminal usage = %+v, want completion=3 (estimated)", last.Usage)
	}
}

// TestOpenAIToResponsesStream_ContentFilterIncomplete content_filter → incomplete/content_filter。
func TestOpenAIToResponsesStream_ContentFilterIncomplete(t *testing.T) {
	sse := strings.Join([]string{
		chatChunk(`{"role":"assistant","content":"I"}`, ""),
		``,
		chatChunk(`{}`, "content_filter"),
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	events, err := runChatToResponsesStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	last := events[len(events)-1]
	if last.Event != "response.incomplete" {
		t.Fatalf("terminal event = %s, want response.incomplete", last.Event)
	}
	details, _ := responseObj(t, last)["incomplete_details"].(map[string]any)
	if details == nil || details["reason"] != "content_filter" {
		t.Errorf("incomplete_details = %v, want reason=content_filter", responseObj(t, last)["incomplete_details"])
	}
}

// TestOpenAIToResponsesStream_Truncated 既无 finish_reason 也无 [DONE] 即 EOF（上游崩溃/代理
// 切断/聚合器静默失败）：不得合成假成功的 response.completed，必须以 ErrUpstreamStreamTruncated
// 返回（宿主据此补发 response.failed），已拿到的 usage 经侧信道（Data 为空的事件）上报。
func TestOpenAIToResponsesStream_Truncated(t *testing.T) {
	sse := strings.Join([]string{
		chatChunk(`{"role":"assistant","content":"Hello wor"}`, ""),
		``,
		`data: {"id":"abc","object":"chat.completion.chunk","created":111,"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":9,"completion_tokens":2,"total_tokens":11}}`,
		``,
	}, "\n")
	events, err := runChatToResponsesStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if !errors.Is(err, relayconvert.ErrUpstreamStreamTruncated) {
		t.Fatalf("err = %v, want ErrUpstreamStreamTruncated", err)
	}
	for _, ev := range events {
		if ev.Event == "response.completed" || ev.Event == "response.incomplete" || ev.Event == "response.failed" {
			t.Fatalf("truncated stream must not synthesize a terminal event, got %s", ev.Event)
		}
	}
	last := events[len(events)-1]
	if last.Data != nil || last.Usage == nil {
		t.Fatalf("last event should be a usage-only side-channel event, got event=%q data=%v usage=%v", last.Event, last.Data, last.Usage)
	}
	if last.Usage.PromptTokens != 9 || last.Usage.CompletionTokens != 2 {
		t.Errorf("side-channel usage = %+v, want prompt=9 completion=2", last.Usage)
	}
}

// TestOpenAIToResponsesStream_DoneWithoutFinishReason 上游只发 [DONE] 不发 finish_reason
// （部分兼容上游）：[DONE] 是显式终止哨兵，按正常 completed 收尾，且文本 part 的 done 事件
// 不得缺失（此前这条路径直接跳到 response.completed，output_text.done 被吞）。
func TestOpenAIToResponsesStream_DoneWithoutFinishReason(t *testing.T) {
	sse := strings.Join([]string{
		chatChunk(`{"role":"assistant","content":"Hi"}`, ""),
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	events, err := runChatToResponsesStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	want := []string{
		"response.created",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.completed",
	}
	got := eventNames(events)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v, want %v", got, want)
	}
	if responseObj(t, events[len(events)-1])["status"] != "completed" {
		t.Errorf("status = %v, want completed", responseObj(t, events[len(events)-1])["status"])
	}
}

// TestOpenAIToResponsesStream_DuplicateFinishReason 上游重复发 finish_reason（n>1 / 某些聚合器）
// 时收口事件只发一次：对已 done 的 item 再发 done 会让客户端状态机崩溃。
func TestOpenAIToResponsesStream_DuplicateFinishReason(t *testing.T) {
	sse := strings.Join([]string{
		chatChunk(`{"role":"assistant","content":"Hi"}`, ""),
		``,
		chatChunk(`{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"f","arguments":"{}"}}]}`, ""),
		``,
		chatChunk(`{}`, "tool_calls"),
		``,
		chatChunk(`{}`, "tool_calls"),
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	events, err := runChatToResponsesStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	counts := map[string]int{}
	for _, ev := range events {
		counts[ev.Event]++
	}
	if counts["response.output_text.done"] != 1 {
		t.Errorf("output_text.done emitted %d times, want 1", counts["response.output_text.done"])
	}
	if counts["response.function_call_arguments.done"] != 1 {
		t.Errorf("function_call_arguments.done emitted %d times, want 1", counts["response.function_call_arguments.done"])
	}
	// message + function_call 各一次 output_item.done
	if counts["response.output_item.done"] != 2 {
		t.Errorf("output_item.done emitted %d times, want 2", counts["response.output_item.done"])
	}
	if counts["response.completed"] != 1 {
		t.Errorf("response.completed emitted %d times, want 1", counts["response.completed"])
	}
}
