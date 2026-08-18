package oai_responses

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

type r2cStreamMeta struct {
	convmeta.Values
	requestID string
}

func (m *r2cStreamMeta) GetRequestID() string  { return m.requestID }
func (m *r2cStreamMeta) ModelNameMapped() bool { return false }

func runR2CStream(t *testing.T, sse string) ([]*dto.ChatCompletionStreamResponse, error) {
	t.Helper()
	meta := &r2cStreamMeta{Values: convmeta.Values{ChannelMetaAttached: true, UpstreamModelName: "gpt-4o"}, requestID: "req-7"}
	var chunks []*dto.ChatCompletionStreamResponse
	err := (&ResponsesToOpenAIChatStreamConverter{}).ConvertStreamResponse(
		context.Background(), meta, strings.NewReader(sse), func(chunk any) error {
			if c, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
				chunks = append(chunks, c)
			}
			return nil
		})
	return chunks, err
}

// 文本流：role 首 chunk → content delta → finish + usage chunk。
func TestR2CStream_Text(t *testing.T) {
	sse := "data: " + `{"type":"response.created","response":{"id":"resp_1","model":"gpt-4o","created_at":1730000000}}` + "\n\n" +
		"data: " + `{"type":"response.output_text.delta","delta":"你好"}` + "\n\n" +
		"data: " + `{"type":"response.output_text.delta","delta":"世界"}` + "\n\n" +
		"data: " + `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-4o","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}` + "\n\n"
	chunks, err := runR2CStream(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	// 序列：role chunk → 2×content delta → finish chunk → usage chunk
	if len(chunks) != 5 {
		t.Fatalf("chunks = %d, want 5", len(chunks))
	}
	if chunks[0].Choices[0].Delta.Role != "assistant" {
		t.Error("首 chunk 应为 role=assistant")
	}
	if chunks[1].Choices[0].Delta.Content != "你好" || chunks[2].Choices[0].Delta.Content != "世界" {
		t.Errorf("content deltas = %v %v", chunks[1].Choices[0].Delta.Content, chunks[2].Choices[0].Delta.Content)
	}
	if chunks[3].Choices[0].FinishReason == nil || *chunks[3].Choices[0].FinishReason != "stop" {
		t.Errorf("finish = %v, want stop", chunks[3].Choices[0].FinishReason)
	}
	if chunks[4].Usage == nil || chunks[4].Usage.TotalTokens != 15 || len(chunks[4].Choices) != 0 {
		t.Errorf("usage chunk = %+v, want 独立 usage chunk total=15", chunks[4])
	}
	if chunks[0].ID != "chatcmpl-req-7" {
		t.Errorf("ID = %q, want chatcmpl-req-7", chunks[0].ID)
	}
	if chunks[1].Model != "gpt-4o" {
		t.Errorf("model = %q（response.created 覆盖）", chunks[1].Model)
	}
}

// 工具流：added 带半截 args → delta 续传 → done 带全量（前缀差分只发增量）。
func TestR2CStream_ToolPrefixDiff(t *testing.T) {
	sse := "data: " + `{"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_1","name":"f","arguments":"{\"ci"}}` + "\n\n" +
		"data: " + `{"type":"response.function_call_arguments.delta","item_id":"call_1","delta":"ty\":\"北京\"}"}` + "\n\n" +
		"data: " + `{"type":"response.output_item.done","item":{"type":"function_call","call_id":"call_1","name":"f","arguments":"{\"city\":\"北京\"}"}}` + "\n\n" +
		"data: " + `{"type":"response.completed","response":{"usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}}}` + "\n\n"
	chunks, err := runR2CStream(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var toolChunks []dto.ToolCall
	for _, c := range chunks {
		if len(c.Choices) > 0 && len(c.Choices[0].Delta.ToolCalls) > 0 {
			toolChunks = append(toolChunks, c.Choices[0].Delta.ToolCalls[0])
		}
	}
	// added（name+首片）、delta（续片）、done（前缀差分→零增量，跳过）
	if len(toolChunks) != 2 {
		t.Fatalf("tool chunks = %d, want 2（done 全量与前缀一致应零增量跳过）", len(toolChunks))
	}
	if toolChunks[0].Function.Name != "f" || toolChunks[0].Function.Arguments != "{\"ci" {
		t.Errorf("首个 tool chunk = %+v, want name=f args={\"ci", toolChunks[0])
	}
	if toolChunks[0].Index != 0 {
		t.Errorf("首个工具 index = %d, want 0（首见从 0 分配）", toolChunks[0].Index)
	}
	if toolChunks[1].Function.Name != "" || toolChunks[1].Function.Arguments != "ty\":\"北京\"}" {
		t.Errorf("delta tool chunk = %+v, want 无 name、args 增量", toolChunks[1])
	}
	// finish 判据：tool + 无文本 → tool_calls
	var finish *string
	for _, c := range chunks {
		if len(c.Choices) > 0 && c.Choices[0].FinishReason != nil {
			finish = c.Choices[0].FinishReason
		}
	}
	if finish == nil || *finish != "tool_calls" {
		t.Errorf("finish = %v, want tool_calls（有工具无文本）", finish)
	}
}

// 工具+文本并存 → finish=stop（legacy 取舍）。
func TestR2CStream_ToolWithTextFinishesStop(t *testing.T) {
	sse := "data: " + `{"type":"response.output_text.delta","delta":"先查一下"}` + "\n\n" +
		"data: " + `{"type":"response.output_item.added","item":{"type":"function_call","call_id":"c1","name":"f","arguments":"{}"}}` + "\n\n" +
		"data: " + `{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}` + "\n\n"
	chunks, err := runR2CStream(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	for _, c := range chunks {
		if len(c.Choices) > 0 && c.Choices[0].FinishReason != nil {
			if *c.Choices[0].FinishReason != "stop" {
				t.Errorf("finish = %s, want stop（有文本有工具——legacy 取舍）", *c.Choices[0].FinishReason)
			}
			return
		}
	}
	t.Fatal("no finish chunk")
}

// reasoning delta → reasoning_content。
func TestR2CStream_Reasoning(t *testing.T) {
	sse := "data: " + `{"type":"response.reasoning_summary_text.delta","delta":"思考中"}` + "\n\n" +
		"data: " + `{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}` + "\n\n"
	chunks, err := runR2CStream(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	found := false
	for _, c := range chunks {
		if len(c.Choices) > 0 && c.Choices[0].Delta.ReasoningContent != nil {
			if *c.Choices[0].Delta.ReasoningContent == "思考中" {
				found = true
			}
		}
	}
	if !found {
		t.Error("reasoning delta 未映射为 reasoning_content")
	}
}

// response.error → EmbeddedUpstreamError。
func TestR2CStream_ErrorEvent(t *testing.T) {
	sse := "data: " + `{"type":"response.error","response":{"error":{"message":"quota","type":"insufficient_quota"}}}` + "\n\n"
	_, err := runR2CStream(t, sse)
	if err == nil {
		t.Fatal("expected error")
	}
	var embedded *types.EmbeddedUpstreamError
	if !errors.As(err, &embedded) {
		t.Fatalf("error should be *EmbeddedUpstreamError, got %T: %v", err, err)
	}
}

// 意外断流（无 completed）：补 start+finish，无 usage chunk。
func TestR2CStream_UnexpectedEOF(t *testing.T) {
	sse := "data: " + `{"type":"response.output_text.delta","delta":"部分"}` + "\n\n"
	chunks, err := runR2CStream(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	hasFinish, hasUsageChunk := false, false
	for _, c := range chunks {
		if len(c.Choices) > 0 && c.Choices[0].FinishReason != nil {
			hasFinish = true
		}
		if c.Usage != nil {
			hasUsageChunk = true
		}
	}
	if !hasFinish {
		t.Error("断流应补 finish chunk")
	}
	if hasUsageChunk {
		t.Error("断流无 usage，不应发 usage chunk（估算兜底归桥接）")
	}
}
