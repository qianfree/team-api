package oai_chat

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// collectClaudeStreamEvents 驱动流式转换器并收集输出的 StreamEvent 序列。
func collectClaudeStreamEvents(t *testing.T, sse string, info convmeta.Meta) ([]*relayconvert.StreamEvent, error) {
	t.Helper()
	converter := &OpenAIToClaudeStreamConverter{}
	var events []*relayconvert.StreamEvent
	err := converter.ConvertStreamResponse(context.Background(), info, strings.NewReader(sse), func(chunk any) error {
		ev, ok := chunk.(*relayconvert.StreamEvent)
		if !ok {
			t.Fatalf("expected *relayconvert.StreamEvent, got %T", chunk)
		}
		events = append(events, ev)
		return nil
	})
	return events, err
}

// eventNames 提取事件名序列。
func eventNames(events []*relayconvert.StreamEvent) []string {
	names := make([]string, 0, len(events))
	for _, ev := range events {
		names = append(names, ev.Event)
	}
	return names
}

// claudePayload 断言事件负载为 *dto.ClaudeResponse 并返回。
func claudePayload(t *testing.T, ev *relayconvert.StreamEvent) *dto.ClaudeResponse {
	t.Helper()
	resp, ok := ev.Data.(*dto.ClaudeResponse)
	if !ok {
		t.Fatalf("event %q payload = %T, want *dto.ClaudeResponse", ev.Event, ev.Data)
	}
	return resp
}

func TestOpenAIToClaudeStreamConverter_Metadata(t *testing.T) {
	converter := &OpenAIToClaudeStreamConverter{}

	if converter.ID() != relayconvert.ResponseConverterOAIChatToClaudeMessagesStream {
		t.Errorf("ID() = %q, want %q", converter.ID(), relayconvert.ResponseConverterOAIChatToClaudeMessagesStream)
	}
	if converter.From() != types.RelayFormatOpenAI {
		t.Errorf("From() = %q, want %q", converter.From(), types.RelayFormatOpenAI)
	}
	if converter.To() != types.RelayFormatClaude {
		t.Errorf("To() = %q, want %q", converter.To(), types.RelayFormatClaude)
	}
	if converter.Quality() != relayconvert.ResponseConverterQualityGood {
		t.Errorf("Quality() = %q, want %q", converter.Quality(), relayconvert.ResponseConverterQualityGood)
	}
}

// TestOpenAIToClaudeStreamConverter_BasicTextStream 纯文本流：事件序列与负载逐项校验，
// 缓存明细按 Claude 语义在 message_delta 补报（input 扣减 cached），
// 宿主捕获的 usage 保持 OpenAI 原始口径且带明细。
func TestOpenAIToClaudeStreamConverter_BasicTextStream(t *testing.T) {
	sse := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":" there"}}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[],"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120,"prompt_tokens_details":{"cached_tokens":30},"completion_tokens_details":{"reasoning_tokens":5}}}

data: [DONE]

`
	events, err := collectClaudeStreamEvents(t, sse, newClaudeInboundMeta())
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	want := []string{"message_start", "content_block_start", "content_block_delta", "content_block_delta", "content_block_stop", "message_delta", "message_stop"}
	got := eventNames(events)
	if len(got) != len(want) {
		t.Fatalf("event sequence = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}

	// message_start：msg_ 前缀 ID、assistant 角色、映射后的模型名（IsModelMapped → upstream）
	start := claudePayload(t, events[0])
	if start.Message == nil || start.Message.Role != "assistant" || start.Message.Type != "message" {
		t.Errorf("message_start payload = %+v", start.Message)
	}
	if !strings.HasPrefix(start.Message.ID, "msg_") {
		t.Errorf("message ID = %q, want msg_ prefix", start.Message.ID)
	}
	if start.Message.Model != "gpt-4o" {
		t.Errorf("message model = %q, want gpt-4o (model mapped → upstream)", start.Message.Model)
	}

	// content_block_start：text 块从空文本开始
	blockStart := claudePayload(t, events[1])
	if blockStart.ContentBlock == nil || blockStart.ContentBlock.Type != "text" {
		t.Errorf("content_block_start = %+v", blockStart.ContentBlock)
	}
	if blockStart.Index == nil || *blockStart.Index != 0 {
		t.Errorf("content_block_start index = %v, want 0", blockStart.Index)
	}

	// 文本 delta
	delta1 := claudePayload(t, events[2])
	if delta1.Delta == nil || delta1.Delta.Type != "text_delta" || delta1.Delta.Text == nil || *delta1.Delta.Text != "Hi" {
		t.Errorf("delta[0] = %+v", delta1.Delta)
	}
	delta2 := claudePayload(t, events[3])
	if delta2.Delta == nil || delta2.Delta.Text == nil || *delta2.Delta.Text != " there" {
		t.Errorf("delta[1] = %+v", delta2.Delta)
	}

	// message_delta：Claude 语义补报（input 扣减 cached）+ stop_reason 映射
	msgDelta := claudePayload(t, events[5])
	if msgDelta.Delta == nil || msgDelta.Delta.StopReason == nil || *msgDelta.Delta.StopReason != "end_turn" {
		t.Errorf("message_delta stop_reason = %+v, want end_turn", msgDelta.Delta)
	}
	if msgDelta.Usage == nil {
		t.Fatal("message_delta usage missing")
	}
	if msgDelta.Usage.InputTokens != 70 {
		t.Errorf("message_delta input_tokens = %d, want 70 (prompt 100 - cached 30)", msgDelta.Usage.InputTokens)
	}
	if msgDelta.Usage.CacheReadInputTokens != 30 {
		t.Errorf("message_delta cache_read_input_tokens = %d, want 30", msgDelta.Usage.CacheReadInputTokens)
	}
	if msgDelta.Usage.OutputTokens != 20 {
		t.Errorf("message_delta output_tokens = %d, want 20", msgDelta.Usage.OutputTokens)
	}

	// 宿主捕获 usage：OpenAI 原始口径 + 明细透传
	if events[5].Usage == nil {
		t.Fatal("message_delta StreamEvent.Usage missing")
	}
	hostUsage := events[5].Usage
	if hostUsage.PromptTokens != 100 || hostUsage.CompletionTokens != 20 || hostUsage.TotalTokens != 120 {
		t.Errorf("host usage = %+v, want prompt=100 completion=20 total=120", hostUsage)
	}
	if hostUsage.PromptTokensDetails == nil || hostUsage.PromptTokensDetails.CachedTokens != 30 {
		t.Errorf("host usage prompt details = %+v, want cached=30", hostUsage.PromptTokensDetails)
	}
	if hostUsage.CompletionTokenDetails == nil || hostUsage.CompletionTokenDetails.ReasoningTokens != 5 {
		t.Errorf("host usage completion details = %+v, want reasoning=5", hostUsage.CompletionTokenDetails)
	}

	// 非 usage 事件不携带宿主用量
	for i, ev := range events {
		if i != 5 && ev.Usage != nil {
			t.Errorf("event[%d] %q carries unexpected usage", i, ev.Event)
		}
	}
}

// TestOpenAIToClaudeStreamConverter_ToolCallStream 文本后接工具调用：
// 关闭文本块 → tool_use content_block_start（含 id/name/空 input）→ input_json_delta。
func TestOpenAIToClaudeStreamConverter_ToolCallStream(t *testing.T) {
	sse := `data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"}}]}

data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}

data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]}}]}

data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"北京\"}"}}]}}]}

data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]

`
	events, err := collectClaudeStreamEvents(t, sse, newClaudeInboundMeta())
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	want := []string{
		"message_start",
		"content_block_start", // text
		"content_block_delta", // "ok"
		"content_block_stop",  // 关闭 text 块
		"content_block_start", // tool_use
		"content_block_delta", // args part1
		"content_block_delta", // args part2
		"content_block_stop",  // [DONE] 关闭 tool_use 块
		"message_delta",
		"message_stop",
	}
	got := eventNames(events)
	if len(got) != len(want) {
		t.Fatalf("event sequence = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}

	// tool_use 块（index 1）：id/name/空 input
	toolStart := claudePayload(t, events[4])
	if toolStart.Index == nil || *toolStart.Index != 1 {
		t.Errorf("tool_use index = %v, want 1", toolStart.Index)
	}
	cb := toolStart.ContentBlock
	if cb == nil || cb.Type != "tool_use" || cb.ID != "call_1" || cb.Name != "get_weather" {
		t.Errorf("tool_use block = %+v", cb)
	}
	if input, ok := cb.Input.(map[string]any); !ok || len(input) != 0 {
		t.Errorf("tool_use input = %v, want empty map", cb.Input)
	}

	// input_json_delta 参数分片
	argsDelta := claudePayload(t, events[5])
	if argsDelta.Delta == nil || argsDelta.Delta.Type != "input_json_delta" ||
		argsDelta.Delta.PartialJSON == nil || *argsDelta.Delta.PartialJSON != `{"city":` {
		t.Errorf("input_json_delta[0] = %+v", argsDelta.Delta)
	}
	argsDelta2 := claudePayload(t, events[6])
	if argsDelta2.Delta == nil || argsDelta2.Delta.PartialJSON == nil || *argsDelta2.Delta.PartialJSON != `"北京"}` {
		t.Errorf("input_json_delta[1] = %+v", argsDelta2.Delta)
	}

	// finish_reason=tool_calls → stop_reason=tool_use
	msgDelta := claudePayload(t, events[8])
	if msgDelta.Delta == nil || msgDelta.Delta.StopReason == nil || *msgDelta.Delta.StopReason != "tool_use" {
		t.Errorf("stop_reason = %+v, want tool_use", msgDelta.Delta)
	}
}

// TestOpenAIToClaudeStreamConverter_ThinkingStream reasoning_content 映射为 thinking 块，
// 思考转正文时补 content_block_stop / content_block_start。
func TestOpenAIToClaudeStreamConverter_ThinkingStream(t *testing.T) {
	sse := `data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"thinking..."}}]}

data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"answer"}}]}

data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]

`
	events, err := collectClaudeStreamEvents(t, sse, newClaudeInboundMeta())
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	want := []string{
		"message_start",
		"content_block_start", // thinking
		"content_block_delta", // thinking_delta
		"content_block_stop",  // 关闭 thinking 块
		"content_block_start", // text
		"content_block_delta", // text_delta
		"content_block_stop",  // [DONE] 关闭 text 块
		"message_delta",
		"message_stop",
	}
	got := eventNames(events)
	if len(got) != len(want) {
		t.Fatalf("event sequence = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}

	thinkStart := claudePayload(t, events[1])
	if thinkStart.ContentBlock == nil || thinkStart.ContentBlock.Type != "thinking" {
		t.Errorf("thinking block start = %+v", thinkStart.ContentBlock)
	}
	thinkDelta := claudePayload(t, events[2])
	if thinkDelta.Delta == nil || thinkDelta.Delta.Type != "thinking_delta" ||
		thinkDelta.Delta.Thinking == nil || *thinkDelta.Delta.Thinking != "thinking..." {
		t.Errorf("thinking_delta = %+v", thinkDelta.Delta)
	}
	textStart := claudePayload(t, events[4])
	if textStart.ContentBlock == nil || textStart.ContentBlock.Type != "text" {
		t.Errorf("text block start = %+v", textStart.ContentBlock)
	}
	if textStart.Index == nil || *textStart.Index != 1 {
		t.Errorf("text block index = %v, want 1", textStart.Index)
	}
}

// TestOpenAIToClaudeStreamConverter_UnexpectedEnd 上游未发 [DONE] 即断流：
// 补发 content_block_stop + message_stop，已收到的 usage 附在 message_stop 上供宿主捕获。
func TestOpenAIToClaudeStreamConverter_UnexpectedEnd(t *testing.T) {
	sse := `data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"partial"}}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}

`
	events, err := collectClaudeStreamEvents(t, sse, newClaudeInboundMeta())
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	want := []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_stop"}
	got := eventNames(events)
	if len(got) != len(want) {
		t.Fatalf("event sequence = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}

	last := events[len(events)-1]
	if last.Usage == nil {
		t.Fatal("message_stop should carry usage on unexpected stream end")
	}
	if last.Usage.PromptTokens != 10 || last.Usage.CompletionTokens != 2 || last.Usage.TotalTokens != 12 {
		t.Errorf("usage = %+v, want prompt=10 completion=2 total=12", last.Usage)
	}
}

// TestOpenAIToClaudeStreamConverter_MalformedJSONSkipped 非法 JSON 行跳过不中断。
func TestOpenAIToClaudeStreamConverter_MalformedJSONSkipped(t *testing.T) {
	sse := `data: {not valid json}

data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}

data: [DONE]

`
	events, err := collectClaudeStreamEvents(t, sse, newClaudeInboundMeta())
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}
	got := eventNames(events)
	want := []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"}
	if len(got) != len(want) {
		t.Fatalf("event sequence = %v, want %v", got, want)
	}
}

// TestOpenAIToClaudeStreamConverter_ModelNotMapped 未映射时 message_start 回显入站模型名。
func TestOpenAIToClaudeStreamConverter_ModelNotMapped(t *testing.T) {
	sse := `data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"x"}}]}

data: [DONE]

`
	info := &convmeta.Values{
		OriginModelName:     "gpt-4o",
		UpstreamModelName:   "gpt-4o",
		ChannelMetaAttached: true,
	}
	events, err := collectClaudeStreamEvents(t, sse, info)
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}
	start := claudePayload(t, events[0])
	if start.Message == nil || start.Message.Model != "gpt-4o" {
		t.Errorf("model = %+v, want gpt-4o", start.Message)
	}
}

// TestOpenAIToClaudeStreamConverter_ContextCancellation ctx 取消时返回 ctx.Err()。
func TestOpenAIToClaudeStreamConverter_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sse := `data: {"id":"c","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"x"}}]}

data: [DONE]

`
	converter := &OpenAIToClaudeStreamConverter{}
	err := converter.ConvertStreamResponse(ctx, newClaudeInboundMeta(), strings.NewReader(sse), func(chunk any) error {
		return nil
	})
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
