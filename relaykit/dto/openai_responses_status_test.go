package dto

import (
	"encoding/json"
	"testing"
)

// Responses 终止态 → chat finish_reason 的映射是反方向（Responses 上游 → chat 客户端）
// 识别截断的唯一依据：漏判即把被 max_output_tokens 截断的回答当成 finish_reason=stop 发出。
func TestOpenAIResponsesResponse_TruncatedFinishReason(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"completed", `{"status":"completed","incomplete_details":null}`, ""},
		{"max_output_tokens", `{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}`, "length"},
		{"content_filter", `{"status":"incomplete","incomplete_details":{"reason":"content_filter"}}`, "content_filter"},
		// incomplete 但 reason 缺失：按绝对主因 length 处理，比假装 stop 更接近真相
		{"incomplete_no_reason", `{"status":"incomplete"}`, "length"},
		{"status_null", `{"status":null}`, ""},
		{"status_missing", `{}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var r OpenAIResponsesResponse
			if err := json.Unmarshal([]byte(tc.body), &r); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := r.TruncatedFinishReason(); got != tc.want {
				t.Errorf("TruncatedFinishReason() = %q, want %q", got, tc.want)
			}
		})
	}

	// nil 接收者不得 panic（流式事件的 Response 字段可能为空）
	var nilResp *OpenAIResponsesResponse
	if got := nilResp.TruncatedFinishReason(); got != "" {
		t.Errorf("nil receiver = %q, want empty", got)
	}
}

// IncompleteReason 兼容 Go 侧构造的结构体形态（值 / 指针）与 JSON 解码后的 map 形态。
func TestOpenAIResponsesResponse_IncompleteReasonShapes(t *testing.T) {
	cases := []struct {
		name    string
		details any
		want    string
	}{
		{"struct", IncompleteDetails{Reason: "max_output_tokens"}, "max_output_tokens"},
		{"pointer", &IncompleteDetails{Reason: "content_filter"}, "content_filter"},
		{"nil_pointer", (*IncompleteDetails)(nil), ""},
		{"map", map[string]any{"reason": "max_output_tokens"}, "max_output_tokens"},
		{"nil", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &OpenAIResponsesResponse{IncompleteDetails: tc.details}
			if got := r.IncompleteReason(); got != tc.want {
				t.Errorf("IncompleteReason() = %q, want %q", got, tc.want)
			}
		})
	}
}
