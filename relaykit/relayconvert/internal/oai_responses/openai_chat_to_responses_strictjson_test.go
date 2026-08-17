package oai_responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

// TestChatToResponsesStrictJSONKeys 回归测试：codex（Rust serde）严格解析 response 对象，
// usage 细分键缺失会解析失败。断言极端场景（无 details、空 content）下的键集合。
func TestChatToResponsesStrictJSONKeys(t *testing.T) {
	// 最简响应：无 usage details、空 content、无工具
	var chatResp dto.ChatCompletionResponse
	if err := json.Unmarshal([]byte(`{
		"id": "chatcmpl-1", "object": "chat.completion", "created": 1730000000, "model": "m",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": ""}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
	}`), &chatResp); err != nil {
		t.Fatalf("parse chat response: %v", err)
	}

	result, _, err := (&OpenAIChatToResponsesResponseConverter{}).ConvertResponse(context.Background(), nil, &chatResp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// usage 细分键：codex 严格解析必需（含零值）
	usage, ok := resp["usage"].(map[string]any)
	if !ok {
		t.Fatalf("usage missing: %v", resp["usage"])
	}
	for _, key := range []string{"input_tokens", "output_tokens", "total_tokens", "input_tokens_details", "output_tokens_details"} {
		if _, exists := usage[key]; !exists {
			t.Errorf("usage.%s missing", key)
		}
	}
	outDetails, _ := usage["output_tokens_details"].(map[string]any)
	if outDetails == nil {
		t.Fatal("output_tokens_details missing")
	}
	for _, key := range []string{"reasoning_tokens", "audio_tokens", "accepted_prediction_tokens", "rejected_prediction_tokens"} {
		if _, exists := outDetails[key]; !exists {
			t.Errorf("output_tokens_details.%s missing", key)
		}
	}
	inDetails, _ := usage["input_tokens_details"].(map[string]any)
	if inDetails == nil {
		t.Fatal("input_tokens_details missing")
	}
	for _, key := range []string{"cached_tokens", "cache_write_tokens", "audio_tokens"} {
		if _, exists := inDetails[key]; !exists {
			t.Errorf("input_tokens_details.%s missing", key)
		}
	}

	// 顶层键：typed 形态恒含 prompt/conversation null 键（P0 已上线口径，与 legacy map 差异为已知无害项）
	for _, key := range []string{"prompt", "conversation", "error", "incomplete_details"} {
		if _, exists := resp[key]; !exists {
			t.Errorf("response.%s missing (typed 形态应恒存在)", key)
		}
	}
}
