package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

// Responses 上游的两条宿主路径（UseResponsesAPI 桥接给 chat 客户端 / codex 直连透传）在上游
// 截断或断流时的收尾语义：截断要映射 finish_reason=length，断流要报错并按流中断结算。

func streamResp(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// chatFinishReasons 提取 chat SSE 输出中出现的全部 finish_reason。
func chatFinishReasons(t *testing.T, body string) []string {
	t.Helper()
	var out []string
	for _, f := range collectSSEDataFrames(body) {
		if f == "[DONE]" {
			continue
		}
		var chunk dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(f), &chunk); err != nil {
			continue
		}
		for _, c := range chunk.Choices {
			if c.FinishReason != nil {
				out = append(out, *c.FinishReason)
			}
		}
	}
	return out
}

// TestHandleResponsesStreamToChat_Incomplete 上游 response.incomplete（max_output_tokens）：
// chat 客户端收到 finish_reason=length，且 incomplete 携带的真实 usage 被用于计费。
func TestHandleResponsesStreamToChat_Incomplete(t *testing.T) {
	upstream := strings.Join([]string{
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"The answer is"}`,
		``,
		"event: response.incomplete",
		`data: {"type":"response.incomplete","response":{"id":"resp_1","object":"response","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"usage":{"input_tokens":20,"output_tokens":64,"total_tokens":84}}}`,
		``,
	}, "\n")
	info := responsesToChatInfo()
	rec := httptest.NewRecorder()

	usage, err := HandleResponsesStreamToChat(context.Background(), streamResp(upstream), info, rec)
	if err != nil {
		t.Fatalf("HandleResponsesStreamToChat: %v", err)
	}
	if frs := chatFinishReasons(t, rec.Body.String()); len(frs) != 1 || frs[0] != "length" {
		t.Errorf("finish_reasons = %v, want [length]", frs)
	}
	if usage == nil || usage.PromptTokens != 20 || usage.CompletionTokens != 64 {
		t.Errorf("usage = %+v, want prompt=20 completion=64", usage)
	}
}

// TestHandleResponsesStreamToChat_Truncated 上游无终止态事件即 EOF：不得发假 finish_reason=stop，
// 改发 error chunk + [DONE]；end reason 细化为 upstream_truncated（流中断），按中断兜底结算，
// 以 502 + ResponseWritten 上报调度。
func TestHandleResponsesStreamToChat_Truncated(t *testing.T) {
	upstream := strings.Join([]string{
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"Hello wor"}`,
		``,
	}, "\n")
	info := responsesToChatInfo()
	info.SetEstimatePromptTokens(42)
	rec := httptest.NewRecorder()

	usage, err := HandleResponsesStreamToChat(context.Background(), streamResp(upstream), info, rec)
	var relayErr *constant.RelayError
	if !errors.As(err, &relayErr) || relayErr.StatusCode != http.StatusBadGateway || !relayErr.ResponseWritten {
		t.Fatalf("err = %v, want 502 RelayError with ResponseWritten", err)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonUpstreamTruncated {
		t.Errorf("end reason = %q, want upstream_truncated", reason)
	}
	if !info.StreamStatus.IsPartialStreamEnd() {
		t.Error("upstream_truncated must count as partial stream end")
	}
	body := rec.Body.String()
	if frs := chatFinishReasons(t, body); len(frs) != 0 {
		t.Errorf("truncated stream must not emit finish_reason, got %v", frs)
	}
	if !strings.Contains(body, `"code":"upstream_truncated"`) || !strings.HasSuffix(strings.TrimSpace(body), "data: [DONE]") {
		t.Errorf("client must receive an error chunk followed by [DONE], body:\n%s", body)
	}
	// "Hello wor" = 9 字节 → (9+1)/2 = 5；输入用请求侧估算值补齐
	if usage == nil || usage.CompletionTokens != 5 || usage.PromptTokens != 42 {
		t.Errorf("usage = %+v, want completion=5 prompt=42 (interrupted fallback)", usage)
	}
}

// TestHandleResponsesNonStreamToChat_Incomplete 非流式 legacy 路径：status=incomplete → length。
func TestHandleResponsesNonStreamToChat_Incomplete(t *testing.T) {
	body := `{"id":"resp_1","object":"response","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},` +
		`"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"The ans"}]}],` +
		`"usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}}`
	info := responsesToChatInfo()
	info.IsStream = false
	rec := httptest.NewRecorder()
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}

	if _, err := HandleResponsesNonStreamToChat(context.Background(), resp, info, rec); err != nil {
		t.Fatalf("HandleResponsesNonStreamToChat: %v", err)
	}
	var chatResp dto.ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &chatResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(chatResp.Choices) != 1 || chatResp.Choices[0].FinishReason != "length" {
		t.Errorf("finish_reason = %+v, want length", chatResp.Choices)
	}
}

func responsesPassthroughInfo() *common.RelayInfo {
	return &common.RelayInfo{
		RequestID:       "test-req-passthrough",
		OriginModelName: "gpt-5-codex",
		IsStream:        true,
		ClientFormat:    constant.RelayFormatResponses,
		InboundFormat:   constant.RelayFormatResponses,
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			UpstreamModelName: "gpt-5-codex",
			SupportsResponses: true,
		},
	}
}

// TestHandleResponsesUpstreamStream_Truncated 直连透传：上游无终止态事件即 EOF。此前原样关流、
// 记 done、按成功计费；现补发 response.failed、记 upstream_truncated、502 上报。
func TestHandleResponsesUpstreamStream_Truncated(t *testing.T) {
	upstream := strings.Join([]string{
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"Hel"}`,
		``,
	}, "\n")
	info := responsesPassthroughInfo()
	info.SetEstimatePromptTokens(42)
	rec := httptest.NewRecorder()

	usage, err := (&Adaptor{}).handleResponsesUpstreamStream(context.Background(), streamResp(upstream), info, rec)
	var relayErr *constant.RelayError
	if !errors.As(err, &relayErr) || relayErr.StatusCode != http.StatusBadGateway || !relayErr.ResponseWritten {
		t.Fatalf("err = %v, want 502 RelayError with ResponseWritten", err)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonUpstreamTruncated {
		t.Errorf("end reason = %q, want upstream_truncated", reason)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: response.output_text.delta") {
		t.Errorf("already-forwarded frame should be in body:\n%s", body)
	}
	if !strings.Contains(body, "event: response.failed") || !strings.Contains(body, "upstream_truncated") {
		t.Errorf("client must receive response.failed, body:\n%s", body)
	}
	// "Hel" = 3 字节 → (3+1)/2 = 2；输入用请求侧估算值补齐
	if usage == nil || usage.CompletionTokens != 2 || usage.PromptTokens != 42 {
		t.Errorf("usage = %+v, want completion=2 prompt=42", usage)
	}
}

// TestHandleResponsesUpstreamStream_IncompleteIsTerminal 直连透传：response.incomplete 是合法
// 终止态，原样透传、不追加 response.failed，且其携带的 usage 用于计费（此前只认 completed/done）。
func TestHandleResponsesUpstreamStream_IncompleteIsTerminal(t *testing.T) {
	upstream := strings.Join([]string{
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"The ans"}`,
		``,
		"event: response.incomplete",
		`data: {"type":"response.incomplete","response":{"id":"resp_1","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"usage":{"input_tokens":20,"output_tokens":64,"total_tokens":84}}}`,
		``,
	}, "\n")
	info := responsesPassthroughInfo()
	rec := httptest.NewRecorder()

	usage, err := (&Adaptor{}).handleResponsesUpstreamStream(context.Background(), streamResp(upstream), info, rec)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonDone {
		t.Errorf("end reason = %q, want done", reason)
	}
	body := rec.Body.String()
	if strings.Contains(body, "response.failed") {
		t.Errorf("incomplete is terminal, must not append response.failed:\n%s", body)
	}
	if usage == nil || usage.PromptTokens != 20 || usage.CompletionTokens != 64 {
		t.Errorf("usage = %+v, want prompt=20 completion=64 (from incomplete event)", usage)
	}
}
