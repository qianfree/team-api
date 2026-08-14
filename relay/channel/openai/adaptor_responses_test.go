package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

func responsesUpstreamInfo(relayMode constant.RelayMode, isStream bool) *common.RelayInfo {
	return &common.RelayInfo{
		RelayMode:       int(relayMode),
		IsStream:        isStream,
		InboundFormat:   constant.RelayFormatResponses,
		ClientFormat:    constant.RelayFormatResponses,
		OriginModelName: "gpt-4o",
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			BaseURL:           "https://upstream.example.com",
			UpstreamModelName: "gpt-4o-upstream",
			IsModelMapped:     false,
			Settings: common.ChannelSettings{
				UpstreamResponses: true,
			},
		},
	}
}

// TestAdaptor_GetRequestURL_ResponsesUpstream 上游声明 Responses 协议时，
// responses / responses/compact 直达 /v1/responses，chat 仍走 /v1/chat/completions。
func TestAdaptor_GetRequestURL_ResponsesUpstream(t *testing.T) {
	cases := []struct {
		mode constant.RelayMode
		want string
	}{
		{constant.RelayModeResponses, "https://upstream.example.com/v1/responses"},
		{constant.RelayModeResponsesCompact, "https://upstream.example.com/v1/responses/compact"},
		{constant.RelayModeChatCompletions, "https://upstream.example.com/v1/chat/completions"},
	}
	for _, c := range cases {
		a := &Adaptor{}
		info := responsesUpstreamInfo(c.mode, false)
		got, err := a.GetRequestURL(info)
		if err != nil {
			t.Fatalf("GetRequestURL(%v) error: %v", c.mode, err)
		}
		if got != c.want {
			t.Errorf("GetRequestURL(%v) = %q, want %q", c.mode, got, c.want)
		}
	}
}

// TestAdaptor_GetRequestURL_ResponsesChatFallback 未声明 Responses 协议时，
// responses 入站仍转换到 /v1/chat/completions（chat-only 上游兜底）。
func TestAdaptor_GetRequestURL_ResponsesChatFallback(t *testing.T) {
	info := responsesUpstreamInfo(constant.RelayModeResponses, false)
	info.ChannelMeta.Settings.UpstreamResponses = false
	a := &Adaptor{}
	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL error: %v", err)
	}
	if want := "https://upstream.example.com/v1/chat/completions"; got != want {
		t.Errorf("GetRequestURL = %q, want %q", got, want)
	}
}

// TestAdaptor_ConvertRequest_ResponsesUpstream 上游声明 Responses 协议时，
// responses 请求保持 Responses 格式：模型名替换、不注入 chat 专属 stream_options/messages。
func TestAdaptor_ConvertRequest_ResponsesUpstream(t *testing.T) {
	info := responsesUpstreamInfo(constant.RelayModeResponses, true)
	info.ChannelMeta.IsModelMapped = true

	body := []byte(`{"model":"gpt-4o","input":"say hi","stream":true}`)
	a := &Adaptor{}
	out, err := a.ConvertRequest(context.Background(), info, body)
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	raw, _ := io.ReadAll(out)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("bad converted json: %v\n%s", err, raw)
	}
	if got := string(m["model"]); got != `"gpt-4o-upstream"` {
		t.Errorf("model = %s, want gpt-4o-upstream", got)
	}
	if _, ok := m["input"]; !ok {
		t.Error("responses body should keep input field")
	}
	if _, ok := m["messages"]; ok {
		t.Error("should NOT inject chat messages field")
	}
	if _, ok := m["stream_options"]; ok {
		t.Error("should NOT inject chat stream_options for responses upstream")
	}
}

// TestAdaptor_ConvertRequest_ResponsesUpstream_Thinking 上游声明 Responses 协议时，
// thinking 后缀映射为 reasoning.effort，不注入 chat 的 reasoning_effort。
func TestAdaptor_ConvertRequest_ResponsesUpstream_Thinking(t *testing.T) {
	info := responsesUpstreamInfo(constant.RelayModeResponses, false)
	info.ReasoningEffort = "high"

	body := []byte(`{"model":"gpt-4o","input":"say hi"}`)
	a := &Adaptor{}
	out, err := a.ConvertRequest(context.Background(), info, body)
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	raw, _ := io.ReadAll(out)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("bad converted json: %v\n%s", err, raw)
	}
	if _, ok := m["reasoning_effort"]; ok {
		t.Error("should NOT inject chat reasoning_effort for responses upstream")
	}
	var reasoning struct {
		Effort string `json:"effort"`
	}
	if err := json.Unmarshal(m["reasoning"], &reasoning); err != nil || reasoning.Effort != "high" {
		t.Errorf("reasoning.effort = %+v (err=%v), want high", reasoning, err)
	}
}

// TestResponsesUsageToCommon 验证 Responses usage → common.Usage 映射。
func TestResponsesUsageToCommon(t *testing.T) {
	u := responsesUsageToCommon(&dto.ResponsesUsage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		InputTokensDetails: &dto.InputTokenDetails{
			CachedTokens: 40,
			AudioTokens:  10,
			ImageTokens:  5,
		},
		OutputTokenDetails: &dto.OutputTokenDetails{
			ReasoningTokens: 20,
			AudioTokens:     3,
		},
	})
	if u.PromptTokens != 100 || u.CompletionTokens != 50 || u.TotalTokens != 150 {
		t.Errorf("bad token counts: %+v", u)
	}
	if !u.CacheIncludedInPrompt {
		t.Error("OpenAI responses usage should set CacheIncludedInPrompt=true")
	}
	if u.PromptTokensDetails == nil || u.PromptTokensDetails.CachedTokens != 40 {
		t.Errorf("cached tokens not mapped: %+v", u.PromptTokensDetails)
	}
	if u.CompletionTokenDetails == nil || u.CompletionTokenDetails.ReasoningTokens != 20 {
		t.Errorf("reasoning tokens not mapped: %+v", u.CompletionTokenDetails)
	}
}

// TestAdaptor_DoResponse_ResponsesUpstreamNonStream 上游 Responses 响应原样透传，usage 正确解析。
func TestAdaptor_DoResponse_ResponsesUpstreamNonStream(t *testing.T) {
	respBody := `{"id":"resp_1","object":"response","status":"completed","model":"gpt-4o-upstream","output":[],"usage":{"input_tokens":100,"output_tokens":50,"total_tokens":150,"input_tokens_details":{"cached_tokens":40}}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(respBody)),
	}

	info := responsesUpstreamInfo(constant.RelayModeResponses, false)
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	usage, err := a.DoResponse(context.Background(), resp, info, rec)
	if err != nil {
		t.Fatalf("DoResponse error: %v", err)
	}
	if got := rec.Body.String(); got != respBody {
		t.Errorf("response body not passthrough:\n got %s\nwant %s", got, respBody)
	}
	if usage.PromptTokens != 100 || usage.CompletionTokens != 50 {
		t.Errorf("usage = %+v, want prompt=100 completion=50", usage)
	}
}

// TestAdaptor_DoResponse_ResponsesUpstreamStream 上游 Responses SSE 逐行透传，usage 从 completed 事件解析。
func TestAdaptor_DoResponse_ResponsesUpstreamStream(t *testing.T) {
	ss := strings.Join([]string{
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"Hello"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed","usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30,"input_tokens_details":{"cached_tokens":5}}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(ss)),
	}

	info := responsesUpstreamInfo(constant.RelayModeResponses, true)
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	usage, err := a.DoResponse(context.Background(), resp, info, rec)
	if err != nil {
		t.Fatalf("DoResponse error: %v", err)
	}
	out := rec.Body.String()
	for _, want := range []string{"event: response.output_text.delta", `"delta":"Hello"`, "event: response.completed", "data: [DONE]"} {
		if !strings.Contains(out, want) {
			t.Errorf("stream output missing %q\n---\n%s", want, out)
		}
	}
	if usage.PromptTokens != 10 || usage.CompletionTokens != 20 || usage.TotalTokens != 30 {
		t.Errorf("usage = %+v, want prompt=10 completion=20 total=30", usage)
	}
}

// TestAdaptor_DoResponse_ResponsesUpstreamNonStream_Error 非 200 上游响应：
// 错误体透传给客户端的同时必须返回 upstream error（驱动重试/渠道健康上报）。
func TestAdaptor_DoResponse_ResponsesUpstreamNonStream_Error(t *testing.T) {
	respBody := `{"error":{"type":"rate_limit_error","message":"Rate limit reached"}}`
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(respBody)),
	}

	info := responsesUpstreamInfo(constant.RelayModeResponses, false)
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	usage, err := a.DoResponse(context.Background(), resp, info, rec)
	if err == nil {
		t.Fatal("non-200 upstream response should return an upstream error, got nil")
	}
	if usage == nil {
		t.Fatal("usage should be non-nil for error path")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "rate_limit_error") {
		t.Errorf("error body not passed through: %s", rec.Body.String())
	}
}

// TestAdaptor_DoResponse_ResponsesUpstreamStream_Error 流式请求但上游在 SSE 开始前
// 返回非 200：同样必须返回 upstream error。
func TestAdaptor_DoResponse_ResponsesUpstreamStream_Error(t *testing.T) {
	respBody := `{"error":{"type":"invalid_request_error","message":"model not found"}}`
	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(respBody)),
	}

	info := responsesUpstreamInfo(constant.RelayModeResponses, true)
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	_, err := a.DoResponse(context.Background(), resp, info, rec)
	if err == nil {
		t.Fatal("non-200 upstream response should return an upstream error, got nil")
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// TestAdaptor_DoResponse_ResponsesUpstreamStream_ModelMapped 模型映射时，
// SSE 事件中的上游模型名须回写为客户端请求的模型名。
func TestAdaptor_DoResponse_ResponsesUpstreamStream_ModelMapped(t *testing.T) {
	ss := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-4o-upstream"}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-4o-upstream","usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}}}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(ss)),
	}

	info := responsesUpstreamInfo(constant.RelayModeResponses, true)
	info.ChannelMeta.IsModelMapped = true
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	usage, err := a.DoResponse(context.Background(), resp, info, rec)
	if err != nil {
		t.Fatalf("DoResponse error: %v", err)
	}
	out := rec.Body.String()
	if strings.Contains(out, "gpt-4o-upstream") {
		t.Errorf("upstream model name leaked to client:\n%s", out)
	}
	if !strings.Contains(out, `"model":"gpt-4o"`) {
		t.Errorf("client-requested model name missing in stream:\n%s", out)
	}
	// 模型名回写不得破坏 usage 解析
	if usage.PromptTokens != 10 || usage.CompletionTokens != 20 {
		t.Errorf("usage = %+v, want prompt=10 completion=20", usage)
	}
}
