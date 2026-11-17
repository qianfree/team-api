package oai_responses

import (
	"errors"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// 反方向（Responses 上游 → chat 客户端）的终止语义：与 chat → Responses 方向对称，
// 截断必须映射为 finish_reason=length / content_filter，断流必须报错而非合成 stop。

// softErrMeta 测试用 Meta：实现 SoftErrorRecorder，收集转换器留下的软错误。
type softErrMeta struct {
	*convmeta.Values
	errs []string
}

func (m *softErrMeta) RecordConvertSoftError(msg string) { m.errs = append(m.errs, msg) }

func lastFinishReason(chunks []*dto.ChatCompletionStreamResponse) string {
	for i := len(chunks) - 1; i >= 0; i-- {
		for _, c := range chunks[i].Choices {
			if c.FinishReason != nil {
				return *c.FinishReason
			}
		}
	}
	return ""
}

// TestResponsesToOpenAIStream_IncompleteLength 上游 response.incomplete（max_output_tokens）：
// chat 客户端必须收到 finish_reason=length。此前不处理 incomplete 事件，走 EOF 兜底发 stop，
// 截断的回答在客户端看来是完整的；且 incomplete 携带的 usage 被丢弃、只能按文本估算。
func TestResponsesToOpenAIStream_IncompleteLength(t *testing.T) {
	sse := strings.Join([]string{
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"The answer is"}`,
		``,
		`event: response.incomplete`,
		`data: {"type":"response.incomplete","response":{"id":"resp_1","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"usage":{"input_tokens":20,"output_tokens":64,"total_tokens":84}}}`,
		``,
	}, "\n")
	chunks, err := runResponsesToChatStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	if fr := lastFinishReason(chunks); fr != "length" {
		t.Errorf("finish_reason = %q, want length", fr)
	}
	// incomplete 的真实 usage 必须透传（而非 13/4 的文本估算）
	last := chunks[len(chunks)-1]
	if last.Usage == nil || last.Usage.PromptTokens != 20 || last.Usage.CompletionTokens != 64 {
		t.Errorf("usage = %+v, want prompt=20 completion=64", last.Usage)
	}
}

// TestResponsesToOpenAIStream_IncompleteContentFilter content_filter → finish_reason=content_filter。
func TestResponsesToOpenAIStream_IncompleteContentFilter(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"I"}`,
		``,
		`data: {"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"content_filter"}}}`,
		``,
	}, "\n")
	chunks, err := runResponsesToChatStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	if fr := lastFinishReason(chunks); fr != "content_filter" {
		t.Errorf("finish_reason = %q, want content_filter", fr)
	}
}

// TestResponsesToOpenAIStream_Truncated 既无终止态事件也无 [DONE] 即 EOF：不得合成
// finish_reason=stop，以 ErrUpstreamStreamTruncated 返回；已转发的增量保留。
func TestResponsesToOpenAIStream_Truncated(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"Hello wor"}`,
		``,
	}, "\n")
	chunks, err := runResponsesToChatStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if !errors.Is(err, relayconvert.ErrUpstreamStreamTruncated) {
		t.Fatalf("err = %v, want ErrUpstreamStreamTruncated", err)
	}
	if fr := lastFinishReason(chunks); fr != "" {
		t.Errorf("truncated stream must not synthesize finish_reason, got %q", fr)
	}
	var text strings.Builder
	for _, c := range chunks {
		for _, ch := range c.Choices {
			if s, ok := ch.Delta.Content.(string); ok {
				text.WriteString(s)
			}
		}
	}
	if text.String() != "Hello wor" {
		t.Errorf("forwarded text = %q, want Hello wor", text.String())
	}
}

// TestResponsesToOpenAIStream_DoneWithoutTerminal 只有 [DONE] 没有终止态事件（部分兼容上游）：
// [DONE] 是显式终止哨兵，按正常 stop 收尾。
func TestResponsesToOpenAIStream_DoneWithoutTerminal(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"Hi"}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	chunks, err := runResponsesToChatStream(t, &convmeta.Values{OriginModelName: "gpt-4o"}, sse)
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	if fr := lastFinishReason(chunks); fr != "stop" {
		t.Errorf("finish_reason = %q, want stop", fr)
	}
}

// TestResponsesToOpenAIStream_SoftErrorRecorded 无法解析的 data 行跳过但必须留痕：
// 此前被静默 continue，线上排障只能逐帧对照调试日志。
func TestResponsesToOpenAIStream_SoftErrorRecorded(t *testing.T) {
	sse := strings.Join([]string{
		`data: {not json`,
		``,
		`data: {"type":"response.output_text.delta","delta":"ok"}`,
		``,
		`data: {"type":"response.completed","response":{"status":"completed"}}`,
		``,
	}, "\n")
	meta := &softErrMeta{Values: &convmeta.Values{OriginModelName: "gpt-4o"}}
	if _, err := runResponsesToChatStream(t, meta, sse); err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	if len(meta.errs) != 1 || !strings.Contains(meta.errs[0], "{not json") {
		t.Errorf("soft errors = %v, want one entry carrying the bad line", meta.errs)
	}
}

// TestOpenAIToResponsesStream_SoftErrorRecorded chat → Responses 方向同样留痕。
func TestOpenAIToResponsesStream_SoftErrorRecorded(t *testing.T) {
	sse := strings.Join([]string{
		`data: {broken`,
		``,
		chatChunk(`{"role":"assistant","content":"Hi"}`, "stop"),
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	meta := &softErrMeta{Values: &convmeta.Values{OriginModelName: "gpt-4o"}}
	if _, err := runChatToResponsesStream(t, meta, sse); err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	if len(meta.errs) != 1 || !strings.Contains(meta.errs[0], "{broken") {
		t.Errorf("soft errors = %v, want one entry carrying the bad line", meta.errs)
	}
}

// TestResponsesToOpenAIResponse_IncompleteFinishReason 非流式：status=incomplete 映射为
// length / content_filter；工具调用 + incomplete 时截断优先（参数可能不完整）。
func TestResponsesToOpenAIResponse_IncompleteFinishReason(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"completed", `{"id":"r","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}]}`, "stop"},
		{"max_output_tokens", `{"id":"r","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"The ans"}]}]}`, "length"},
		{"content_filter", `{"id":"r","status":"incomplete","incomplete_details":{"reason":"content_filter"},"output":[]}`, "content_filter"},
		{"tool_call_truncated", `{"id":"r","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"function_call","call_id":"c1","name":"f","arguments":"{\"a\":"}]}`, "length"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chatResp := convertResponsesResponse(t, &convmeta.Values{OriginModelName: "gpt-4o"}, tc.body)
			if got := chatResp.Choices[0].FinishReason; got != tc.want {
				t.Errorf("finish_reason = %q, want %q", got, tc.want)
			}
		})
	}
}
