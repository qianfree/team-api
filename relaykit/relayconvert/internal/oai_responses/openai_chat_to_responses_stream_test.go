package oai_responses

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/types"
)

// runStreamCollector 冻结时钟并收集 chunk。
func runStreamCollector(t *testing.T, sse string) ([]*dto.ResponsesStreamEvent, error) {
	t.Helper()
	originalNow := NowFunc
	NowFunc = func() time.Time { return time.Unix(1730000000, 0) }
	defer func() { NowFunc = originalNow }()

	var events []*dto.ResponsesStreamEvent
	err := (&OpenAIChatToResponsesStreamConverter{}).ConvertStreamResponse(
		context.Background(), nil, strings.NewReader(sse), func(chunk any) error {
			if event, ok := chunk.(*dto.ResponsesStreamEvent); ok {
				events = append(events, event)
			}
			return nil
		})
	return events, err
}

// 假成功防护：上游流不是 chat 格式时返回 ErrProtocolMismatch 包装错误。
func TestChatStream_ProtocolMismatch(t *testing.T) {
	// Responses 格式的 SSE（能解析为空 chat chunk 但无 choices）
	sse := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_x\"}}\n\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"hi\"}\n\n"
	_, err := runStreamCollector(t, sse)
	if err == nil {
		t.Fatal("expected protocol mismatch error")
	}
	if !errors.Is(err, types.ErrProtocolMismatch) {
		t.Errorf("error should wrap ErrProtocolMismatch, got: %v", err)
	}
}

// SSE 内嵌上游错误返回 *EmbeddedUpstreamError 且保留原文。
func TestChatStream_EmbeddedUpstreamError(t *testing.T) {
	sse := "data: {\"error\":{\"message\":\"quota exceeded\",\"type\":\"insufficient_quota\"}}\n\n"
	_, err := runStreamCollector(t, sse)
	if err == nil {
		t.Fatal("expected embedded upstream error")
	}
	var embedded *types.EmbeddedUpstreamError
	if !errors.As(err, &embedded) {
		t.Fatalf("error should be *EmbeddedUpstreamError, got %T: %v", err, err)
	}
	if !strings.Contains(string(embedded.Body), "insufficient_quota") {
		t.Errorf("embedded body lost upstream detail: %s", embedded.Body)
	}
}

// 正常 chunk 携带 "error":null 不应误判为内嵌错误。
func TestChatStream_NullErrorFieldIgnored(t *testing.T) {
	sse := "data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":\"stop\"}],\"error\":null}\n\n" +
		"data: [DONE]\n\n"
	events, err := runStreamCollector(t, sse)
	if err != nil {
		t.Fatalf("null error field should be ignored: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected events emitted")
	}
}

// 空流（无任何 data 行）触发协议不匹配防护。
func TestChatStream_EmptyStream(t *testing.T) {
	_, err := runStreamCollector(t, "")
	if !errors.Is(err, types.ErrProtocolMismatch) {
		t.Errorf("empty stream should trigger mismatch, got: %v", err)
	}
}

// [DONE]-only 流同样触发防护（无任何有效内容）。
func TestChatStream_DoneOnlyStream(t *testing.T) {
	_, err := runStreamCollector(t, "data: [DONE]\n\n")
	if !errors.Is(err, types.ErrProtocolMismatch) {
		t.Errorf("[DONE]-only stream should trigger mismatch, got: %v", err)
	}
}

// 多工具 done 事件按登记顺序发出（确定性修复项的显式断言）。
func TestChatStream_ToolDoneOrder(t *testing.T) {
	sse := "data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[" +
		"{\"index\":0,\"id\":\"call_2nd\",\"type\":\"function\",\"function\":{\"name\":\"b\",\"arguments\":\"\"}}]}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[" +
		"{\"index\":1,\"id\":\"call_1st\",\"type\":\"function\",\"function\":{\"name\":\"a\",\"arguments\":\"\"}}]}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: [DONE]\n\n"
	events, err := runStreamCollector(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var doneOrder []string
	for _, e := range events {
		if e.Type == "response.function_call_arguments.done" {
			if data, ok := e.Data.(map[string]any); ok {
				doneOrder = append(doneOrder, data["item_id"].(string))
			}
		}
	}
	if len(doneOrder) != 2 || doneOrder[0] != "call_2nd" || doneOrder[1] != "call_1st" {
		t.Errorf("tool done order = %v, want [call_2nd call_1st] (登记顺序)", doneOrder)
	}
}

// 重复 finish_reason 不重复发 done（确定性修复项的显式断言）。
func TestChatStream_RepeatedFinishNoDuplicateDone(t *testing.T) {
	sse := "data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[" +
		"{\"index\":0,\"id\":\"call_x\",\"type\":\"function\",\"function\":{\"name\":\"f\",\"arguments\":\"{}\"}}]}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
	events, err := runStreamCollector(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	argsDone, itemDone := 0, 0
	for _, e := range events {
		if e.Type == "response.function_call_arguments.done" {
			argsDone++
		}
		if e.Type == "response.output_item.done" {
			if data, ok := e.Data.(map[string]any); ok {
				if item, ok := data["item"].(map[string]any); ok && item["type"] == "function_call" {
					itemDone++
				}
			}
		}
	}
	if argsDone != 1 || itemDone != 1 {
		t.Errorf("duplicate done events: args.done=%d item.done=%d, want 1/1", argsDone, itemDone)
	}
}

// 参数 chunk 仅带 index（无 ID）时按 index 反查到已登记的 callID。
func TestChatStream_ArgsChunkByIndex(t *testing.T) {
	sse := "data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[" +
		"{\"index\":3,\"id\":\"call_idx\",\"type\":\"function\",\"function\":{\"name\":\"f\",\"arguments\":\"\"}}]}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[" +
		"{\"index\":3,\"function\":{\"arguments\":\"{\\\"x\\\":1}\"}}]}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: [DONE]\n\n"
	events, err := runStreamCollector(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	found := false
	for _, e := range events {
		if e.Type == "response.function_call_arguments.delta" {
			if data, ok := e.Data.(map[string]any); ok {
				if data["item_id"] == "call_idx" && data["delta"] == "{\"x\":1}" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("args chunk with index only should be routed to call_idx via index lookup")
	}
}

// 上游无 usage 时 completed 事件内嵌 len/4 估算（客户端可见口径）。
func TestChatStream_UsageEstimateEmbedded(t *testing.T) {
	sse := "data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"12345678\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
	events, err := runStreamCollector(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	for _, e := range events {
		if e.Type != "response.completed" {
			continue
		}
		data := e.Data.(map[string]any)
		resp := data["response"].(*dto.OpenAIResponsesResponse)
		if resp.Usage == nil || resp.Usage.OutputTokens != 2 { // 8 字节 / 4
			t.Errorf("completed usage output_tokens = %+v, want 2 (len/4 估算)", resp.Usage)
		}
	}
}
