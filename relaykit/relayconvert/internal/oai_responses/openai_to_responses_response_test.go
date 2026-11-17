package oai_responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// convertChatToResponses 驱动非流式 chat → Responses 转换并返回 JSON 解码后的 map。
func convertChatToResponses(t *testing.T, chatResp *dto.ChatCompletionResponse) map[string]any {
	t.Helper()
	conv := &OpenAIToResponsesResponseConverter{}
	out, err := conv.ConvertResponse(context.Background(), &convmeta.Values{OriginModelName: "gpt-4o"}, chatResp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func chatRespWithFinish(finishReason string) *dto.ChatCompletionResponse {
	return &dto.ChatCompletionResponse{
		ID:      "abc",
		Object:  "chat.completion",
		Created: 111,
		Model:   "gpt-4o",
		Choices: []dto.Choice{{
			Index:        0,
			Message:      dto.Message{Role: "assistant", Content: "partial answer"},
			FinishReason: finishReason,
		}},
		Usage: dto.UsageWithDetails{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
	}
}

// TestOpenAIToResponsesResponse_TerminalState finish_reason → Responses 终止态映射：
// length / content_filter 必须是 incomplete + incomplete_details，否则被 max_tokens 截断的
// 回答在 codex 看来是完整答案；stop 仍为 completed。
func TestOpenAIToResponsesResponse_TerminalState(t *testing.T) {
	cases := []struct {
		finish       string
		wantStatus   string
		wantReason   string // 空 = incomplete_details 应为 null
		wantItemStat string
	}{
		{"stop", "completed", "", "completed"},
		{"", "completed", "", "completed"},
		{"tool_calls", "completed", "", "completed"},
		{"length", "incomplete", "max_output_tokens", "incomplete"},
		{"content_filter", "incomplete", "content_filter", "incomplete"},
	}
	for _, tc := range cases {
		t.Run("finish="+tc.finish, func(t *testing.T) {
			m := convertChatToResponses(t, chatRespWithFinish(tc.finish))
			if m["status"] != tc.wantStatus {
				t.Errorf("status = %v, want %s", m["status"], tc.wantStatus)
			}
			details, _ := m["incomplete_details"].(map[string]any)
			if tc.wantReason == "" {
				if m["incomplete_details"] != nil {
					t.Errorf("incomplete_details = %v, want null", m["incomplete_details"])
				}
			} else if details == nil || details["reason"] != tc.wantReason {
				t.Errorf("incomplete_details = %v, want reason=%s", m["incomplete_details"], tc.wantReason)
			}
			output, _ := m["output"].([]any)
			if len(output) != 1 {
				t.Fatalf("output len = %d, want 1", len(output))
			}
			msg, _ := output[0].(map[string]any)
			if msg["status"] != tc.wantItemStat {
				t.Errorf("message status = %v, want %s", msg["status"], tc.wantItemStat)
			}
			// 文本内容不因终止态而丢失
			content, _ := msg["content"].([]any)
			if len(content) != 1 {
				t.Fatalf("content len = %d, want 1", len(content))
			}
			if part, _ := content[0].(map[string]any); part["text"] != "partial answer" {
				t.Errorf("text = %v, want partial answer", part["text"])
			}
		})
	}
}
