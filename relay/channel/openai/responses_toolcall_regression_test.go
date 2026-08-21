package openai

// 回归测试：ChatViaResponses 流式工具调用的 item_id/call_id 键归一
// （relaykit 孪生修复在宿主路径的同步，审查发现，修复后固化）。

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

// TestHandleResponsesStreamToChat_ToolCallItemIDMismatch 回归：Responses 上游的
// response.function_call_arguments.delta 只携带 item_id（output item 的 id，如 fc_xxx，
// ≠ call_id call_xxx）。不归一到同一键时，delta 事件分配新 index，name 与参数碎裂成
// 两个 tool_call，done 事件再按首个 index 全量重发参数——客户端组装出非法 JSON。
func TestHandleResponsesStreamToChat_ToolCallItemIDMismatch(t *testing.T) {
	ss := strings.Join([]string{
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","item":{"id":"fc_1","call_id":"call_1","type":"function_call","name":"lookup","arguments":""}}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"a\""}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":":1}"}`,
		"",
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","item":{"id":"fc_1","call_id":"call_1","type":"function_call","name":"lookup","arguments":"{\"a\":1}"}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":5,"output_tokens":7,"total_tokens":12}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(ss)),
	}

	info := responsesUpstreamInfo(constant.RelayModeChatCompletions, true)
	info.RequestID = "req-toolcall-1"
	rec := httptest.NewRecorder()
	if _, err := HandleResponsesStreamToChat(context.Background(), resp, info, rec); err != nil {
		t.Fatalf("HandleResponsesStreamToChat error: %v", err)
	}

	// 解析输出 SSE，聚合 tool_calls 的 index 与参数
	indexes := map[int]bool{}
	argsByID := map[int]string{}
	namesByID := map[int]string{}
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") || strings.Contains(line, "[DONE]") {
			continue
		}
		var chunk dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(line[len("data: "):]), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			for _, tc := range choice.Delta.ToolCalls {
				indexes[tc.Index] = true
				argsByID[tc.Index] += tc.Function.Arguments
				if tc.Function.Name != "" {
					namesByID[tc.Index] = tc.Function.Name
				}
			}
		}
	}

	if len(indexes) != 1 {
		t.Errorf("tool_call index 集合 = %v, want 单一 index（delta 未归一到 call_id 键则碎裂成两个）", indexes)
	}
	for idx, args := range argsByID {
		if want := `{"a":1}`; args != want {
			t.Errorf("index %d 参数拼接 = %q, want %q（done 全量重发会拼出重复参数）", idx, args, want)
		}
	}
	for idx, name := range namesByID {
		if name != "lookup" {
			t.Errorf("index %d name = %q, want lookup", idx, name)
		}
	}
	if len(namesByID) == 0 {
		t.Error("未发出任何带 name 的 tool_call chunk")
	}
}
