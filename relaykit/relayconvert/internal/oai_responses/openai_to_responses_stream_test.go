package oai_responses

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// runChatToResponsesStream 执行 chat SSE → Responses 事件流转换并收集全部 StreamEvent。
func runChatToResponsesStream(t *testing.T, info convmeta.Meta, sse string) ([]*relayconvert.StreamEvent, error) {
	t.Helper()
	conv := &OpenAIToResponsesStreamConverter{}
	var events []*relayconvert.StreamEvent
	err := conv.ConvertStreamResponse(context.Background(), info, strings.NewReader(sse), func(chunk any) error {
		ev, ok := chunk.(*relayconvert.StreamEvent)
		if !ok {
			t.Fatalf("expected *relayconvert.StreamEvent, got %T", chunk)
		}
		events = append(events, ev)
		return nil
	})
	return events, err
}

func eventNames(events []*relayconvert.StreamEvent) []string {
	names := make([]string, 0, len(events))
	for _, ev := range events {
		names = append(names, ev.Event)
	}
	return names
}

func eventData(t *testing.T, ev *relayconvert.StreamEvent) map[string]any {
	t.Helper()
	m, ok := ev.Data.(map[string]any)
	if !ok {
		t.Fatalf("event %s data is %T, want map", ev.Event, ev.Data)
	}
	return m
}

// TestOpenAIToResponsesStream_TextToolCallsUsage 完整链路：文本 + 工具调用 + usage。
// 断言事件名顺序、关键负载与最终 usage（含缓存明细透传）。
func TestOpenAIToResponsesStream_TextToolCallsUsage(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"id":"abc","object":"chat.completion.chunk","created":111,"model":"gpt-4o-upstream","choices":[{"index":0,"delta":{"role":"assistant","content":"Hel"}}]}`,
		``,
		`data: {"id":"abc","object":"chat.completion.chunk","created":111,"model":"gpt-4o-upstream","choices":[{"index":0,"delta":{"content":"lo"}}]}`,
		``,
		`data: {"id":"abc","object":"chat.completion.chunk","created":111,"model":"gpt-4o-upstream","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`,
		``,
		`data: {"id":"abc","object":"chat.completion.chunk","created":111,"model":"gpt-4o-upstream","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"sh\"}"}}]}}]}`,
		``,
		`data: {"id":"abc","object":"chat.completion.chunk","created":111,"model":"gpt-4o-upstream","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		``,
		`data: {"id":"abc","object":"chat.completion.chunk","created":111,"model":"gpt-4o-upstream","choices":[],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10,"prompt_tokens_details":{"cached_tokens":4},"completion_tokens_details":{"reasoning_tokens":2}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	events, err := runChatToResponsesStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}

	wantNames := []string{
		"response.created",
		"response.output_item.added",  // message
		"response.content_part.added", // 空文本 part
		"response.output_text.delta",  // "Hel"
		"response.output_text.delta",  // "lo"
		"response.output_text.done",   // 工具调用开始前关闭文本 part
		"response.content_part.done",
		"response.output_item.done",  // message completed
		"response.output_item.added", // function_call
		"response.function_call_arguments.delta",
		"response.function_call_arguments.done", // finish_reason 触发
		"response.output_item.done",             // function_call completed
		"response.completed",
	}
	gotNames := eventNames(events)
	if len(gotNames) != len(wantNames) {
		t.Fatalf("event count = %d, want %d\ngot: %v", len(gotNames), len(wantNames), gotNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("event[%d] = %s, want %s\ngot: %v", i, gotNames[i], wantNames[i], gotNames)
		}
	}

	// response.created 负载：ID/created 取自首个 chunk；未映射时模型名取上游 chunk 的模型名；
	// store 恒 false；echo 缺失回退默认 temperature/top_p=1.0
	created := eventData(t, events[0])
	if created["type"] != "response.created" {
		t.Errorf("created type = %v", created["type"])
	}
	respObj, _ := created["response"].(map[string]any)
	if respObj["id"] != "resp_abc" || respObj["status"] != "in_progress" {
		t.Errorf("created response = %v", respObj)
	}
	if respObj["created_at"] != 111 && respObj["created_at"] != float64(111) {
		t.Errorf("created_at = %v, want 111", respObj["created_at"])
	}
	if respObj["model"] != "gpt-4o-upstream" {
		t.Errorf("model = %v, want gpt-4o-upstream（未映射时用上游模型名）", respObj["model"])
	}
	if respObj["store"] != false {
		t.Errorf("store = %v, want false", respObj["store"])
	}
	if respObj["temperature"] != 1.0 {
		t.Errorf("temperature = %v, want 1.0 (echo 默认值)", respObj["temperature"])
	}

	// 文本 delta 负载
	delta1 := eventData(t, events[3])
	if delta1["delta"] != "Hel" || delta1["item_id"] != "msg_abc" {
		t.Errorf("text delta = %v", delta1)
	}

	// output_text.done 携带完整文本
	textDone := eventData(t, events[5])
	if textDone["text"] != "Hello" {
		t.Errorf("output_text.done text = %v, want Hello", textDone["text"])
	}

	// function_call added 负载
	fcAdded := eventData(t, events[8])
	fcItem, _ := fcAdded["item"].(map[string]any)
	if fcItem["type"] != "function_call" || fcItem["call_id"] != "call_1" || fcItem["name"] != "get_weather" || fcItem["status"] != "in_progress" {
		t.Errorf("function_call added item = %v", fcItem)
	}
	if fcAdded["output_index"] != 1 {
		t.Errorf("function_call output_index = %v, want 1（文本消息占 0）", fcAdded["output_index"])
	}

	// arguments delta / done
	argsDelta := eventData(t, events[9])
	if argsDelta["delta"] != `{"city":"sh"}` || argsDelta["item_id"] != "call_1" {
		t.Errorf("arguments delta = %v", argsDelta)
	}
	argsDone := eventData(t, events[10])
	if argsDone["arguments"] != `{"city":"sh"}` {
		t.Errorf("arguments done = %v", argsDone)
	}
	fcDone := eventData(t, events[11])
	fcDoneItem, _ := fcDone["item"].(map[string]any)
	if fcDoneItem["status"] != "completed" || fcDoneItem["arguments"] != `{"city":"sh"}` || fcDoneItem["name"] != "get_weather" {
		t.Errorf("function_call done item = %v", fcDoneItem)
	}

	// response.completed：output 含消息 + 工具调用，usage 数值与明细透传
	completed := eventData(t, events[12])
	respDone, _ := completed["response"].(map[string]any)
	if respDone["status"] != "completed" {
		t.Errorf("completed status = %v", respDone["status"])
	}
	output, _ := respDone["output"].([]map[string]any)
	if len(output) != 2 {
		t.Fatalf("completed output = %v, want message + function_call", respDone["output"])
	}
	usageObj, _ := respDone["usage"].(map[string]any)
	if usageObj["input_tokens"] != 7 || usageObj["output_tokens"] != 3 || usageObj["total_tokens"] != 10 {
		t.Errorf("usage map = %v", usageObj)
	}
	inDetails, _ := usageObj["input_tokens_details"].(map[string]any)
	if inDetails["cached_tokens"] != 4 {
		t.Errorf("input_tokens_details = %v（缓存明细不能丢）", inDetails)
	}
	outDetails, _ := usageObj["output_tokens_details"].(map[string]any)
	if outDetails["reasoning_tokens"] != 2 {
		t.Errorf("output_tokens_details = %v", outDetails)
	}

	// StreamEvent.Usage 只挂在 completed 事件上，且带明细
	for i, ev := range events[:len(events)-1] {
		if ev.Usage != nil {
			t.Errorf("event[%d] %s should not carry usage", i, ev.Event)
		}
	}
	final := events[len(events)-1]
	if final.Usage == nil || final.Usage.PromptTokens != 7 || final.Usage.CompletionTokens != 3 || final.Usage.TotalTokens != 10 {
		t.Fatalf("StreamEvent.Usage = %+v", final.Usage)
	}
	if final.Usage.PromptTokensDetails == nil || final.Usage.PromptTokensDetails.CachedTokens != 4 {
		t.Errorf("StreamEvent.Usage prompt details = %+v", final.Usage.PromptTokensDetails)
	}
	if final.Usage.CompletionTokenDetails == nil || final.Usage.CompletionTokenDetails.ReasoningTokens != 2 {
		t.Errorf("StreamEvent.Usage completion details = %+v", final.Usage.CompletionTokenDetails)
	}
}

// TestOpenAIToResponsesStream_TextOnlyEstimatedUsage 纯文本流、上游无 usage：
// finish_reason 触发文本收尾事件，completed usage 按文本长度估算（4 字符/token）。
func TestOpenAIToResponsesStream_TextOnlyEstimatedUsage(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"12345678"}}]}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	events, err := runChatToResponsesStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	names := eventNames(events)
	want := []string{
		"response.created", "response.output_item.added", "response.content_part.added",
		"response.output_text.delta",
		"response.output_text.done", "response.content_part.done", "response.output_item.done",
		"response.completed",
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v, want %v", names, want)
	}
	final := events[len(events)-1]
	if final.Usage == nil || final.Usage.CompletionTokens != 2 || final.Usage.TotalTokens != 2 {
		t.Errorf("estimated usage = %+v, want completion=2 (8 字符 / 4)", final.Usage)
	}
}

// TestOpenAIToResponsesStream_ReasoningDelta chat reasoning_content →
// response.reasoning_summary_text.delta。
func TestOpenAIToResponsesStream_ReasoningDelta(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"mull"}}]}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"hi"}}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	events, err := runChatToResponsesStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	var found bool
	for _, ev := range events {
		if ev.Event == "response.reasoning_summary_text.delta" {
			data := eventData(t, ev)
			if data["delta"] != "mull" || data["summary_index"] != 0 {
				t.Errorf("reasoning delta = %v", data)
			}
			found = true
		}
	}
	if !found {
		t.Error("reasoning_summary_text.delta not found")
	}
}

// TestOpenAIToResponsesStream_EchoFromStash 已 stash 的请求快照 echo 进 response 对象。
func TestOpenAIToResponsesStream_EchoFromStash(t *testing.T) {
	info := newStashMeta("gpt-4o", "", false)
	temp := 0.3
	topP := 0.9
	maxOut := uint(512)
	info.stashed = &dto.OpenAIResponsesRequest{
		Temperature:     &temp,
		TopP:            &topP,
		MaxOutputTokens: &maxOut,
		Instructions:    []byte(`"be nice"`),
	}
	sse := strings.Join([]string{
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"hi"}}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	events, err := runChatToResponsesStream(t, info, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	created := eventData(t, events[0])
	respObj, _ := created["response"].(map[string]any)
	if respObj["temperature"] != 0.3 || respObj["top_p"] != 0.9 {
		t.Errorf("echo temperature/top_p = %v/%v", respObj["temperature"], respObj["top_p"])
	}
	if mo, _ := respObj["max_output_tokens"].(*int); mo == nil || *mo != 512 {
		t.Errorf("echo max_output_tokens = %v", respObj["max_output_tokens"])
	}
}

// TestOpenAIToResponsesStream_EmbeddedError SSE 中内嵌 {"error":{...}} 时终止并报错
// （错误透传由宿主负责）。
func TestOpenAIToResponsesStream_EmbeddedError(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"error":{"message":"quota exceeded","type":"insufficient_quota"}}`,
		``,
	}, "\n")
	events, err := runChatToResponsesStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err == nil {
		t.Fatal("embedded error should terminate the stream with error")
	}
	if !strings.Contains(err.Error(), "quota exceeded") {
		t.Errorf("error = %v", err)
	}
	if len(events) != 0 {
		t.Errorf("no event should be emitted before embedded error, got %v", eventNames(events))
	}
}

// TestOpenAIToResponsesStream_NonChatFormatGuard 上游流不是 chat 格式（无 choices、
// 无内容、无 usage）时不得合成假成功的空 completed，必须报错。
func TestOpenAIToResponsesStream_NonChatFormatGuard(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"leaked responses sse"}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	_, err := runChatToResponsesStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err == nil {
		t.Fatal("non-chat stream should be rejected")
	}
	if !strings.Contains(err.Error(), "not chat completions format") {
		t.Errorf("error = %v", err)
	}
}
