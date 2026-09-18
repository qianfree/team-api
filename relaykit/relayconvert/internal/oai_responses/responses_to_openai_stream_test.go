package oai_responses

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// runResponsesToChatStream 执行 Responses SSE → chat chunk 流式转换并收集全部输出 chunk。
func runResponsesToChatStream(t *testing.T, info convmeta.Meta, sse string) ([]*dto.ChatCompletionStreamResponse, error) {
	t.Helper()
	conv := &ResponsesToOpenAIStreamConverter{}
	var chunks []*dto.ChatCompletionStreamResponse
	err := conv.ConvertStreamResponse(context.Background(), info, strings.NewReader(sse), func(chunk any) error {
		c, ok := chunk.(*dto.ChatCompletionStreamResponse)
		if !ok {
			t.Fatalf("expected *dto.ChatCompletionStreamResponse, got %T", chunk)
		}
		chunks = append(chunks, c)
		return nil
	})
	return chunks, err
}

// TestResponsesToOpenAIStream_TextWithUsage 文本流 + usage：
// 首帧 role/content 空串 → 文本 delta → stop → 末尾 usage chunk（含缓存明细透传）。
func TestResponsesToOpenAIStream_TextWithUsage(t *testing.T) {
	sse := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_up","object":"response","created_at":123,"status":"in_progress","model":"gpt-4o-real"}}`,
		``,
		`data: {"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"Hel"}`,
		``,
		`data: {"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"lo"}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_up","object":"response","created_at":123,"status":"completed","model":"gpt-4o-real","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15,"input_tokens_details":{"cached_tokens":2},"output_tokens_details":{"reasoning_tokens":1}}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	chunks, err := runResponsesToChatStream(t, &convmeta.Values{UpstreamModelName: "gpt-4o", ChannelMetaAttached: true, OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	// 序列：start(role) + delta("Hel") + delta("lo") + stop + usage
	if len(chunks) != 5 {
		t.Fatalf("chunks = %d, want 5", len(chunks))
	}
	start := chunks[0]
	if start.Choices[0].Delta.Role != "assistant" || start.Choices[0].Delta.Content != "" {
		t.Errorf("start chunk = %+v", start.Choices[0].Delta)
	}
	if start.Object != "chat.completion.chunk" {
		t.Errorf("object = %q", start.Object)
	}
	// response.created 覆盖模型名与创建时间
	if start.Model != "gpt-4o-real" || start.Created != 123 {
		t.Errorf("model/created = %v/%v, want gpt-4o-real/123", start.Model, start.Created)
	}
	if chunks[1].Choices[0].Delta.Content != "Hel" || chunks[2].Choices[0].Delta.Content != "lo" {
		t.Errorf("text deltas = %v / %v", chunks[1].Choices[0].Delta.Content, chunks[2].Choices[0].Delta.Content)
	}
	stop := chunks[3]
	if stop.Choices[0].FinishReason == nil || *stop.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason = %v, want stop", stop.Choices[0].FinishReason)
	}
	usageChunk := chunks[4]
	if len(usageChunk.Choices) != 0 || usageChunk.Usage == nil {
		t.Fatalf("usage chunk = %+v", usageChunk)
	}
	if usageChunk.Usage.PromptTokens != 10 || usageChunk.Usage.CompletionTokens != 5 || usageChunk.Usage.TotalTokens != 15 {
		t.Errorf("usage = %+v", usageChunk.Usage)
	}
	if d := usageChunk.Usage.PromptTokensDetails; d == nil || d.CachedTokens != 2 {
		t.Errorf("prompt details = %+v（缓存明细不能丢）", usageChunk.Usage.PromptTokensDetails)
	}
	if d := usageChunk.Usage.CompletionTokenDetails; d == nil || d.ReasoningTokens != 1 {
		t.Errorf("completion details = %+v", usageChunk.Usage.CompletionTokenDetails)
	}
}

// TestResponsesToOpenAIStream_ToolCalls 工具调用流：output_item.added 携带 name，
// function_call_arguments.delta 增量透传（arguments 键恒存在，codex serde 兼容），
// 纯工具调用 finish_reason 为 tool_calls。
func TestResponsesToOpenAIStream_ToolCalls(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"get_weather","arguments":""}}`,
		``,
		`data: {"type":"response.function_call_arguments.delta","item_id":"call_1","output_index":0,"delta":"{\"city\":"}`,
		``,
		`data: {"type":"response.function_call_arguments.delta","item_id":"call_1","output_index":0,"delta":"\"sh\"}"}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_up","object":"response","created_at":123,"status":"completed","usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	chunks, err := runResponsesToChatStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	// 序列：tool chunk(name) + args delta ×2 + start + stop + usage
	// （首个事件为 output_item.added 时 start 帧在 completed 时补发）
	var toolChunks []*dto.ChatCompletionStreamResponse
	var stopChunk *dto.ChatCompletionStreamResponse
	for _, c := range chunks {
		if len(c.Choices) > 0 && len(c.Choices[0].Delta.ToolCalls) > 0 {
			toolChunks = append(toolChunks, c)
		}
		if len(c.Choices) > 0 && c.Choices[0].FinishReason != nil {
			stopChunk = c
		}
	}
	if len(toolChunks) != 3 {
		t.Fatalf("tool chunks = %d, want 3", len(toolChunks))
	}
	first := toolChunks[0].Choices[0].Delta.ToolCalls[0]
	if first.ID != "call_1" || first.Type != "function" || first.Function.Name != "get_weather" || first.Index == nil || *first.Index != 0 {
		t.Errorf("first tool chunk = %+v", first)
	}
	// name 只在首个 chunk 下发一次
	second := toolChunks[1].Choices[0].Delta.ToolCalls[0]
	if second.Function.Name != "" || second.Function.Arguments != `{"city":` {
		t.Errorf("second tool chunk = %+v", second)
	}
	third := toolChunks[2].Choices[0].Delta.ToolCalls[0]
	if third.Function.Arguments != `"sh"}` {
		t.Errorf("third tool chunk = %+v", third)
	}
	if stopChunk == nil || *stopChunk.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("stop chunk = %+v, want finish_reason=tool_calls", stopChunk)
	}
	last := chunks[len(chunks)-1]
	if last.Usage == nil || last.Usage.TotalTokens != 10 {
		t.Errorf("usage chunk = %+v", last)
	}
}

// TestResponsesToOpenAIStream_ReasoningDelta 推理摘要 delta 转 reasoning_content。
func TestResponsesToOpenAIStream_ReasoningDelta(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"response.reasoning_summary_text.delta","item_id":"msg_1","output_index":0,"summary_index":0,"delta":"thinking..."}`,
		``,
		`data: {"type":"response.completed","response":{"id":"r","created_at":1,"status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	chunks, err := runResponsesToChatStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	var found bool
	for _, c := range chunks {
		if len(c.Choices) > 0 && c.Choices[0].Delta.ReasoningContent != nil && *c.Choices[0].Delta.ReasoningContent == "thinking..." {
			found = true
		}
	}
	if !found {
		t.Error("reasoning delta chunk not found")
	}
}

// TestResponsesToOpenAIStream_UsageEstimated 上游未返回 usage 时按累计文本估算（4 字符/token）。
func TestResponsesToOpenAIStream_UsageEstimated(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"response.output_text.delta","item_id":"m","output_index":0,"content_index":0,"delta":"12345678"}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	chunks, err := runResponsesToChatStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	last := chunks[len(chunks)-1]
	if last.Usage == nil || last.Usage.CompletionTokens != 2 || last.Usage.TotalTokens != 2 {
		t.Errorf("estimated usage = %+v, want completion=2 (8 字符 / 4)", last.Usage)
	}
	// 上游没有 completed 事件也要补 stop 帧
	var sawStop bool
	for _, c := range chunks {
		if len(c.Choices) > 0 && c.Choices[0].FinishReason != nil && *c.Choices[0].FinishReason == "stop" {
			sawStop = true
		}
	}
	if !sawStop {
		t.Error("missing fallback stop chunk")
	}
}

// TestResponsesToOpenAIStream_FailedEvent response.failed / response.error 事件终止转换并报错。
func TestResponsesToOpenAIStream_FailedEvent(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"response.failed","response":{"id":"r","created_at":1,"status":"failed","error":{"code":"server_error","message":"boom"}}}`,
		``,
	}, "\n")
	_, err := runResponsesToChatStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err == nil {
		t.Fatal("response.failed should terminate the stream with error")
	}
	if !strings.Contains(err.Error(), "response.failed") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v", err)
	}
}
