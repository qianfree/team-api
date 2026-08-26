package oai_responses

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// ========== 请求侧：reasoning 项还原为 reasoning_content ==========

// 真实 OpenAI 项序（reasoning → function_call → function_call_output）：
// 思考文本附着到 tool_calls 所在的 assistant 消息（DeepSeek 思考模式回传要求）。
func TestR2C_ReasoningAttachedToToolCallMessage(t *testing.T) {
	req := mustParseResponsesRequest(t, `{
		"model": "deepseek-v4-flash",
		"input": [
			{"type":"message","role":"user","content":[{"type":"input_text","text":"查天气"}]},
			{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"需要调用天气工具"}]},
			{"type":"function_call","call_id":"call_1","name":"get_weather","arguments":"{\"city\":\"北京\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"{\"temp\":25}"}
		]
	}`)
	info := &recordingReporter{Values: convmeta.Values{ChannelMetaAttached: true, UpstreamModelName: "deepseek-v4-flash"}}

	result, err := (&ResponsesToOpenAIChatRequestConverter{}).ConvertRequest(context.Background(), info, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	chat := mustCastChatRequest(t, result)

	if len(chat.Messages) != 3 {
		t.Fatalf("messages = %d, want 3 (user/assistant/tool): %+v", len(chat.Messages), chat.Messages)
	}
	assistant := chat.Messages[1]
	if assistant.Role != "assistant" || len(assistant.ToolCalls) != 1 {
		t.Fatalf("assistant message = %+v, want role=assistant with 1 tool_call", assistant)
	}
	if assistant.ReasoningContent == nil || *assistant.ReasoningContent != "需要调用天气工具" {
		t.Errorf("ReasoningContent = %v, want 需要调用天气工具", assistant.ReasoningContent)
	}
	if len(info.reports) != 0 {
		t.Errorf("可还原的 reasoning 项不应上报降级, got %v", info.reports)
	}
}

// 本网关 completed output 项序（message → reasoning → function_call）：
// 同一助手轮合并为单条 assistant 消息（content + reasoning_content + tool_calls）。
func TestR2C_ReasoningAfterMessageMergedIntoSingleTurn(t *testing.T) {
	req := mustParseResponsesRequest(t, `{
		"model": "deepseek-v4-flash",
		"input": [
			{"type":"message","role":"user","content":[{"type":"input_text","text":"查天气"}]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"我来查一下"}]},
			{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"用户要天气"}]},
			{"type":"function_call","call_id":"call_1","name":"get_weather","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_1","output":"晴"}
		]
	}`)

	result, err := (&ResponsesToOpenAIChatRequestConverter{}).ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	chat := mustCastChatRequest(t, result)

	if len(chat.Messages) != 3 {
		t.Fatalf("messages = %d, want 3 (同轮合并为单条 assistant): %+v", len(chat.Messages), chat.Messages)
	}
	assistant := chat.Messages[1]
	if assistant.Role != "assistant" {
		t.Fatalf("messages[1].Role = %s, want assistant", assistant.Role)
	}
	if assistant.Content != "我来查一下" {
		t.Errorf("assistant content = %v, want 我来查一下", assistant.Content)
	}
	if len(assistant.ToolCalls) != 1 || assistant.ToolCalls[0].ID != "call_1" {
		t.Errorf("assistant tool_calls = %+v, want call_1 合并进同条消息", assistant.ToolCalls)
	}
	if assistant.ReasoningContent == nil || *assistant.ReasoningContent != "用户要天气" {
		t.Errorf("ReasoningContent = %v, want 用户要天气", assistant.ReasoningContent)
	}
	if chat.Messages[2].Role != "tool" {
		t.Errorf("messages[2].Role = %s, want tool", chat.Messages[2].Role)
	}
}

// content（reasoning_text，完整思考）优先于 summary。
func TestR2C_ReasoningContentPreferredOverSummary(t *testing.T) {
	req := mustParseResponsesRequest(t, `{
		"model": "m",
		"input": [
			{"type":"reasoning","id":"rs_1",
				"summary":[{"type":"summary_text","text":"摘要"}],
				"content":[{"type":"reasoning_text","text":"完整思考"}]},
			{"type":"function_call","call_id":"c1","name":"f","arguments":"{}"}
		]
	}`)
	result, err := (&ResponsesToOpenAIChatRequestConverter{}).ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	chat := mustCastChatRequest(t, result)
	if len(chat.Messages) != 1 || chat.Messages[0].ReasoningContent == nil {
		t.Fatalf("messages = %+v, want 1 assistant with reasoning", chat.Messages)
	}
	if *chat.Messages[0].ReasoningContent != "完整思考" {
		t.Errorf("ReasoningContent = %q, want 完整思考 (content 优先)", *chat.Messages[0].ReasoningContent)
	}
}

// 无处附着的思考文本（其后无所属 assistant 消息）按 orphan 上报，不误附到下一轮。
func TestR2C_OrphanReasoningReported(t *testing.T) {
	req := mustParseResponsesRequest(t, `{
		"model": "m",
		"input": [
			{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"孤儿思考"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}
		]
	}`)
	info := &recordingReporter{Values: convmeta.Values{}}
	result, err := (&ResponsesToOpenAIChatRequestConverter{}).ConvertRequest(context.Background(), info, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	chat := mustCastChatRequest(t, result)
	for _, msg := range chat.Messages {
		if msg.ReasoningContent != nil {
			t.Errorf("孤儿思考不应附着到任何消息: %+v", msg)
		}
	}
	found := false
	for _, r := range info.reports {
		if r.reason == "input_item:reasoning_orphan" {
			found = true
		}
	}
	if !found {
		t.Errorf("orphan reasoning 应上报降级, got %v", info.reports)
	}
}

// 相邻 assistant 消息与 function_call 之间隔了其他角色消息时不合并（跨轮不串）。
func TestR2C_NoMergeAcrossTurnBoundary(t *testing.T) {
	req := mustParseResponsesRequest(t, `{
		"model": "m",
		"input": [
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"上一轮回答"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"继续"}]},
			{"type":"function_call","call_id":"c1","name":"f","arguments":"{}"}
		]
	}`)
	result, err := (&ResponsesToOpenAIChatRequestConverter{}).ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	chat := mustCastChatRequest(t, result)
	if len(chat.Messages) != 3 {
		t.Fatalf("messages = %d, want 3 (不得跨 user 合并): %+v", len(chat.Messages), chat.Messages)
	}
	if len(chat.Messages[0].ToolCalls) != 0 {
		t.Errorf("上一轮 assistant 消息不应被合入 tool_calls: %+v", chat.Messages[0])
	}
	if chat.Messages[2].Role != "assistant" || len(chat.Messages[2].ToolCalls) != 1 {
		t.Errorf("messages[2] = %+v, want 独立 assistant tool_calls 消息", chat.Messages[2])
	}
}

// ========== 响应侧（流式）：reasoning_content 产出独立 reasoning 项 ==========

// DeepSeek 思考流（先 reasoning 后 content）：reasoning 项在 message 项之前
// （output_index 0/1），事件生命周期完整，completed 的 output 含思考段。
func TestChatStream_ReasoningItemLifecycle(t *testing.T) {
	sse := "data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"思考A\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"思考B\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"答案\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
	events, err := runStreamCollector(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	var seq []string
	var rsAddedIndex, msgAddedIndex = -1, -1
	for _, e := range events {
		data, _ := e.Data.(map[string]any)
		switch e.Type {
		case "response.output_item.added":
			item, _ := data["item"].(map[string]any)
			itemType, _ := item["type"].(string)
			seq = append(seq, "added:"+itemType)
			if itemType == "reasoning" {
				rsAddedIndex = data["output_index"].(int)
				// codex 严格解析：reasoning 项必须携带 summary 键
				if _, ok := item["summary"]; !ok {
					t.Error("reasoning output_item.added 缺少 summary 键（codex serde 必填）")
				}
			}
			if itemType == "message" {
				msgAddedIndex = data["output_index"].(int)
			}
		case "response.reasoning_summary_text.delta":
			seq = append(seq, "rs_delta")
			if id, _ := data["item_id"].(string); !strings.HasPrefix(id, "rs_") {
				t.Errorf("reasoning delta item_id = %q, want rs_ 前缀", id)
			}
		case "response.reasoning_summary_text.done":
			seq = append(seq, "rs_done")
			if text, _ := data["text"].(string); text != "思考A思考B" {
				t.Errorf("reasoning done text = %q, want 思考A思考B", text)
			}
		case "response.output_item.done":
			item, _ := data["item"].(map[string]any)
			seq = append(seq, "done:"+item["type"].(string))
		}
	}

	// reasoning 项先于 message 项（对齐真实 OpenAI 项序）
	if rsAddedIndex != 0 || msgAddedIndex != 1 {
		t.Errorf("output_index: reasoning=%d message=%d, want 0/1", rsAddedIndex, msgAddedIndex)
	}
	wantSeq := []string{"added:reasoning", "rs_delta", "rs_delta", "rs_done", "done:reasoning", "added:message", "done:message"}
	if strings.Join(seq, ",") != strings.Join(wantSeq, ",") {
		t.Errorf("event seq = %v, want %v", seq, wantSeq)
	}

	// completed 的 output：[reasoning, message] 按 output_index 排序
	last := events[len(events)-1]
	if last.Type != "response.completed" {
		t.Fatalf("last event = %s, want response.completed", last.Type)
	}
	resp := last.Data.(map[string]any)["response"].(*dto.OpenAIResponsesResponse)
	if len(resp.Output) != 2 || resp.Output[0].Type != "reasoning" || resp.Output[1].Type != "message" {
		t.Fatalf("completed output = %+v, want [reasoning, message]", resp.Output)
	}
	if len(resp.Output[0].Summary) != 1 || resp.Output[0].Summary[0].Text != "思考A思考B" {
		t.Errorf("reasoning summary = %+v, want 思考A思考B", resp.Output[0].Summary)
	}
}

// 思考 + 工具调用流（无文本）：不合成空 message 项，completed 的 output 为 [reasoning, function_call]。
func TestChatStream_ReasoningWithToolsNoEmptyMessage(t *testing.T) {
	sse := "data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"要调工具\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[" +
		"{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"f\",\"arguments\":\"{}\"}}]}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: [DONE]\n\n"
	events, err := runStreamCollector(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	last := events[len(events)-1]
	resp := last.Data.(map[string]any)["response"].(*dto.OpenAIResponsesResponse)
	if len(resp.Output) != 2 || resp.Output[0].Type != "reasoning" || resp.Output[1].Type != "function_call" {
		t.Fatalf("completed output = %+v, want [reasoning, function_call]（无空 message）", resp.Output)
	}
	// 事件流中也不得出现 message 项
	for _, e := range events {
		if e.Type != "response.output_item.added" {
			continue
		}
		if item, _ := e.Data.(map[string]any)["item"].(map[string]any); item["type"] == "message" {
			t.Error("思考+工具流不应合成空 message 项")
		}
	}
}

// 无思考内容的普通流不产出 reasoning 项（回归保护）。
func TestChatStream_NoReasoningNoExtraItems(t *testing.T) {
	sse := "data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hi\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
	events, err := runStreamCollector(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	for _, e := range events {
		if e.Type == "response.output_item.added" {
			if item, _ := e.Data.(map[string]any)["item"].(map[string]any); item["type"] == "reasoning" {
				t.Error("无思考内容不应产出 reasoning 项")
			}
		}
	}
	last := events[len(events)-1]
	resp := last.Data.(map[string]any)["response"].(*dto.OpenAIResponsesResponse)
	if len(resp.Output) != 1 || resp.Output[0].Type != "message" {
		t.Errorf("completed output = %+v, want 仅 [message]", resp.Output)
	}
}

// ========== 响应侧（非流式）：reasoning_content / thinking 块产出 reasoning 项 ==========

func TestChatResponse_ReasoningItemEmitted(t *testing.T) {
	var chatResp dto.ChatCompletionResponse
	if err := json.Unmarshal([]byte(`{
		"id": "chatcmpl-1", "object": "chat.completion", "created": 1730000000, "model": "deepseek-v4-flash",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "答案", "reasoning_content": "思考过程"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
	}`), &chatResp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, _, err := (&OpenAIChatToResponsesResponseConverter{}).ConvertResponse(context.Background(), nil, &chatResp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	resp := result.(*dto.OpenAIResponsesResponse)
	if len(resp.Output) != 2 || resp.Output[0].Type != "reasoning" || resp.Output[1].Type != "message" {
		t.Fatalf("output = %+v, want [reasoning, message]", resp.Output)
	}
	if resp.Output[0].ID != "rs_chatcmpl-1" ||
		len(resp.Output[0].Summary) != 1 || resp.Output[0].Summary[0].Text != "思考过程" {
		t.Errorf("reasoning item = %+v, want id=rs_chatcmpl-1 text=思考过程", resp.Output[0])
	}
}

func TestClaudeResponse_ThinkingBlockToReasoningItem(t *testing.T) {
	var claudeResp dto.ClaudeResponse
	if err := json.Unmarshal([]byte(`{
		"id": "msg_01", "type": "message", "role": "assistant", "model": "claude-x",
		"content": [
			{"type": "thinking", "thinking": "内部推理", "signature": "sig"},
			{"type": "text", "text": "回答"},
			{"type": "tool_use", "id": "toolu_1", "name": "f", "input": {}}
		],
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`), &claudeResp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, _, err := (&ClaudeToResponsesResponseConverter{}).ConvertResponse(context.Background(), nil, &claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	resp := result.(*dto.OpenAIResponsesResponse)
	if len(resp.Output) != 3 ||
		resp.Output[0].Type != "reasoning" || resp.Output[1].Type != "message" || resp.Output[2].Type != "function_call" {
		t.Fatalf("output = %+v, want [reasoning, message, function_call]", resp.Output)
	}
	if len(resp.Output[0].Summary) != 1 || resp.Output[0].Summary[0].Text != "内部推理" {
		t.Errorf("reasoning summary = %+v, want 内部推理", resp.Output[0].Summary)
	}
}

// ========== 响应侧（Claude 流式）：thinking_delta 产出独立 reasoning 项 ==========

func TestClaudeStream_ThinkingDeltaToReasoningItem(t *testing.T) {
	originalNow := NowFunc
	NowFunc = func() time.Time { return time.Unix(1730000000, 0) }
	defer func() { NowFunc = originalNow }()

	sse := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_01\",\"model\":\"claude-x\",\"usage\":{\"input_tokens\":10,\"output_tokens\":1}}}\n\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\"}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"推理中\"}}\n\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"data: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"text\"}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"回答\"}}\n\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":1}\n\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n\n" +
		"data: {\"type\":\"message_stop\"}\n\n"

	var events []*dto.ResponsesStreamEvent
	err := (&ClaudeToResponsesStreamConverter{}).ConvertStreamResponse(
		context.Background(), nil, strings.NewReader(sse), func(chunk any) error {
			if event, ok := chunk.(*dto.ResponsesStreamEvent); ok {
				events = append(events, event)
			}
			return nil
		})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	sawReasoningAdded, sawReasoningDone := false, false
	for _, e := range events {
		data, _ := e.Data.(map[string]any)
		if e.Type == "response.output_item.added" {
			if item, _ := data["item"].(map[string]any); item["type"] == "reasoning" {
				sawReasoningAdded = true
				if data["output_index"].(int) != 0 {
					t.Errorf("reasoning output_index = %v, want 0（先于 message）", data["output_index"])
				}
			}
		}
		if e.Type == "response.reasoning_summary_text.done" {
			sawReasoningDone = true
			if text, _ := data["text"].(string); text != "推理中" {
				t.Errorf("reasoning done text = %q, want 推理中", text)
			}
		}
	}
	if !sawReasoningAdded || !sawReasoningDone {
		t.Errorf("reasoning 项事件缺失: added=%v done=%v", sawReasoningAdded, sawReasoningDone)
	}

	last := events[len(events)-1]
	if last.Type != "response.completed" {
		t.Fatalf("last event = %s, want response.completed", last.Type)
	}
	resp := last.Data.(map[string]any)["response"].(*dto.OpenAIResponsesResponse)
	if len(resp.Output) != 2 || resp.Output[0].Type != "reasoning" || resp.Output[1].Type != "message" {
		t.Fatalf("completed output = %+v, want [reasoning, message]", resp.Output)
	}
}
