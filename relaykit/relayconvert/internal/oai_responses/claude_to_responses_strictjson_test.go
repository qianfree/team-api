package oai_responses

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relaykit/dto"
)

// TestCompletedEventStrictJSONKeys 回归测试：codex（Rust serde）对 response.completed
// 的 usage 做严格解析——output_tokens_details.reasoning_tokens 等键缺失会直接
// "failed to parse ResponseCompleted: missing field reasoning_tokens" 并触发客户端重连。
// 断言合成事件的关键键在零值场景下也必须存在。
func TestCompletedEventStrictJSONKeys(t *testing.T) {
	originalNow := NowFunc
	NowFunc = func() time.Time { return time.Unix(1730000000, 0) }
	defer func() { NowFunc = originalNow }()

	// 无缓存、无 usage 细分的极端场景：Claude 上游只给了最简 usage
	sse := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"m1\",\"model\":\"glm-4.6\",\"usage\":{\"input_tokens\":10,\"output_tokens\":1}}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}\n\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{},\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}\n\n" +
		"data: {\"type\":\"message_stop\"}\n\n"

	var chunks []any
	err := (&ClaudeToResponsesStreamConverter{}).ConvertStreamResponse(
		context.Background(), nil, strings.NewReader(sse), func(chunk any) error {
			chunks = append(chunks, chunk)
			return nil
		})
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}

	var completedData map[string]any
	found := false
	for _, chunk := range chunks {
		event, ok := chunk.(*dto.ResponsesStreamEvent)
		if !ok {
			continue
		}
		if event.Type == "response.completed" {
			found = true
			completedData, ok = event.Data.(map[string]any)
			if !ok {
				t.Fatalf("response.completed data is %T, want map[string]any", event.Data)
			}
		}
	}
	if !found {
		t.Fatal("no response.completed event emitted")
	}

	respJSON, err := json.Marshal(completedData["response"])
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(respJSON, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// usage 细分键：codex 严格解析必需
	usage, ok := resp["usage"].(map[string]any)
	if !ok {
		t.Fatalf("usage missing or not object: %v", resp["usage"])
	}
	for _, key := range []string{"input_tokens", "output_tokens", "total_tokens", "input_tokens_details", "output_tokens_details"} {
		if _, exists := usage[key]; !exists {
			t.Errorf("usage.%s missing (codex strict parse requires it)", key)
		}
	}
	outDetails, _ := usage["output_tokens_details"].(map[string]any)
	if outDetails == nil {
		t.Fatalf("output_tokens_details missing or not object: %v", usage["output_tokens_details"])
	}
	for _, key := range []string{"reasoning_tokens", "audio_tokens", "accepted_prediction_tokens", "rejected_prediction_tokens"} {
		if _, exists := outDetails[key]; !exists {
			t.Errorf("usage.output_tokens_details.%s missing (codex strict parse requires it)", key)
		}
	}
	inDetails, _ := usage["input_tokens_details"].(map[string]any)
	if inDetails == nil {
		t.Fatalf("input_tokens_details missing or not object: %v", usage["input_tokens_details"])
	}
	for _, key := range []string{"cached_tokens", "cache_write_tokens", "audio_tokens"} {
		if _, exists := inDetails[key]; !exists {
			t.Errorf("usage.input_tokens_details.%s missing (codex strict parse requires it)", key)
		}
	}

	// output 消息项内容块的 annotations 恒存在
	output, _ := resp["output"].([]any)
	if len(output) == 0 {
		t.Fatal("output empty")
	}
	msg, _ := output[0].(map[string]any)
	content, _ := msg["content"].([]any)
	if len(content) == 0 {
		t.Fatal("message content empty")
	}
	part, _ := content[0].(map[string]any)
	if _, exists := part["annotations"]; !exists {
		t.Error("output_text content part missing annotations key")
	}
}
