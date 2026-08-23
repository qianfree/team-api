package oai_chat

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// streamTestMeta 带能力接口的测试桩。
type streamTestMeta struct {
	convmeta.Values
	requestID string
}

func (m *streamTestMeta) GetRequestID() string  { return m.requestID }
func (m *streamTestMeta) ModelNameMapped() bool { return false }

func runO2CStream(t *testing.T, sse string) ([]*dto.ClaudeStreamEvent, error) {
	t.Helper()
	meta := &streamTestMeta{Values: convmeta.Values{ChannelMetaAttached: true, OriginModelName: "glm-4.6"}, requestID: "req-1"}
	var events []*dto.ClaudeStreamEvent
	err := (&OpenAIToClaudeStreamConverter{}).ConvertStreamResponse(
		context.Background(), meta, strings.NewReader(sse), func(chunk any) error {
			if e, ok := chunk.(*dto.ClaudeStreamEvent); ok {
				events = append(events, e)
			}
			return nil
		})
	return events, err
}

// 完整流：文本→思考→工具→finish→usage→[DONE]，验证事件序列与收尾。
func TestO2CStream_FullSequence(t *testing.T) {
	sse := "data: {\"id\":\"c1\",\"model\":\"glm-4.6\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"你好\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"思考\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"t1\",\"type\":\"function\",\"function\":{\"name\":\"f\",\"arguments\":\"\"}}]}}]}\n\n" +
		"data: " + `{"id":"c1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"a\":1}"}}]}}]}` + "\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":20,\"total_tokens\":120,\"prompt_tokens_details\":{\"cached_tokens\":30}}}\n\n" +
		"data: [DONE]\n\n"
	events, err := runO2CStream(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var types []string
	for _, e := range events {
		types = append(types, e.Type)
	}
	// 期望序列：message_start → (text 块三事件) → (关 text 开 thinking 三事件) → (关 thinking 开 tool_use 两事件)
	// → 参数 delta → 关块 → message_delta → message_stop
	want := []string{
		"message_start",
		"content_block_start", "content_block_delta", // text 你好
		"content_block_stop",
		"content_block_start", "content_block_delta", // thinking
		"content_block_stop",
		"content_block_start", // tool_use t1
		"content_block_delta", // 参数 {"a":1}
		"content_block_stop",
		"message_delta",
		"message_stop",
	}
	if len(types) != len(want) {
		t.Fatalf("event sequence = %v\nwant %v", types, want)
	}
	for i := range types {
		if types[i] != want[i] {
			t.Fatalf("event %d = %s, want %s\nfull: %v", i, types[i], want[i], types)
		}
	}
	// message_delta 的 usage 扣减口径：input=100-30=70、cache_read=30、output=20
	last := events[len(events)-2] // message_delta
	if last.Data.Usage == nil || last.Data.Usage.InputTokens != 70 ||
		last.Data.Usage.CacheReadInputTokens != 30 || last.Data.Usage.OutputTokens != 20 {
		t.Errorf("message_delta usage = %+v, want input=70 cache_read=30 output=20", last.Data.Usage)
	}
	// stop_reason = tool_calls→tool_use
	if last.Data.Delta == nil || last.Data.Delta.StopReason == nil || *last.Data.Delta.StopReason != "tool_use" {
		t.Errorf("stop_reason = %+v, want tool_use", last.Data.Delta)
	}
}

// 修复项验证：参数 delta 按 tc.Index 反查所属块——第二个工具已开后，第一个工具的
// 晚到参数应挂回第一个块（legacy 会错挂到当前块=第二个）。
func TestO2CStream_ArgsIndexLookup(t *testing.T) {
	sse := "data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"ta\",\"type\":\"function\",\"function\":{\"name\":\"fa\",\"arguments\":\"\"}}]}}]}\n\n" +
		"data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":1,\"id\":\"tb\",\"type\":\"function\",\"function\":{\"name\":\"fb\",\"arguments\":\"\"}}]}}]}\n\n" +
		// ta 的晚到参数（index=0）——应挂回 ta 的块，而非当前块（tb）
		"data: " + `{"id":"c","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"x\":1}"}}]}}]}` + "\n\n" +
		"data: [DONE]\n\n"
	events, err := runO2CStream(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var argsDeltas []*dto.ClaudeStreamEvent
	for _, e := range events {
		if e.Type == "content_block_delta" && e.Data.Delta != nil && e.Data.Delta.Type == "input_json_delta" {
			argsDeltas = append(argsDeltas, e)
		}
	}
	if len(argsDeltas) != 1 {
		t.Fatalf("args delta count = %d, want 1", len(argsDeltas))
	}
	if argsDeltas[0].Data.Index == nil || *argsDeltas[0].Data.Index != 0 {
		// 块序：contentIndex 从 0 起，ta 块=0、tb 块=1。晚到的 index=0 参数应挂回块 0（ta 的块）。
		t.Errorf("late args delta block index = %v, want 0（ta 的块）", argsDeltas[0].Data.Index)
	}
}

// 修复项验证：意外断流（无 [DONE]）仍补发 message_delta + message_stop。
func TestO2CStream_UnexpectedEOF_GetsMessageDelta(t *testing.T) {
	sse := "data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"部分\"}}]}\n\n" +
		"data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"id\":\"c\",\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}\n\n"
	// 无 [DONE]，直接 EOF
	events, err := runO2CStream(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	hasDelta, hasStop := false, false
	for _, e := range events {
		if e.Type == "message_delta" {
			hasDelta = true
		}
		if e.Type == "message_stop" {
			hasStop = true
		}
	}
	if !hasDelta || !hasStop {
		t.Errorf("意外断流应补 message_delta+message_stop（修复项），got delta=%v stop=%v", hasDelta, hasStop)
	}
}

// 空流：不产出任何事件（调用方按假成功防护处理）。
func TestO2CStream_EmptyStream(t *testing.T) {
	events, err := runO2CStream(t, "")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("empty stream should emit nothing, got %d events", len(events))
	}
}

// TestO2CStream_MultiChoiceGuard n>1 多 choice 流只处理首个 choice：
// 第二 choice 的文本/finish_reason 不得混入输出（交错会损坏 Claude 单消息块流）。
func TestO2CStream_MultiChoiceGuard(t *testing.T) {
	sse := "data: {\"id\":\"c1\",\"model\":\"glm-4.6\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}},{\"index\":1,\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"首选\"}},{\"index\":1,\"delta\":{\"content\":\"次选污染\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":1,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":2,\"total_tokens\":12}}\n\n" +
		"data: [DONE]\n\n"
	events, err := runO2CStream(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	for _, e := range events {
		if e.Data != nil && e.Data.Delta != nil && e.Data.Delta.Text != nil && *e.Data.Delta.Text == "次选污染" {
			t.Error("second choice content must not leak into claude stream")
		}
	}
	// 第二 choice 的 finish_reason 不覆盖：stop_reason 按收尾映射仍应为 stop（无 length 污染）
}
