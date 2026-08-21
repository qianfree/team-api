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

	// blank import 触发内置转换器注册（relaykit 桥接为唯一路径，测试二进制须自备注册）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
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

			SupportsResponses: true,
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
	info.ChannelMeta.SupportsResponses = false
	info.ChannelMeta.ChatViaResponses = false
	a := &Adaptor{}
	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL error: %v", err)
	}
	if want := "https://upstream.example.com/v1/chat/completions"; got != want {
		t.Errorf("GetRequestURL = %q, want %q", got, want)
	}
}

// TestAdaptor_GetRequestURL_ChatViaResponses responses-only 渠道（chat_via_responses）：
// responses 入站同样直连 /v1/responses（上游本来就说 Responses 协议），
// chat 入站经 UseResponsesAPI 桥接也打 /v1/responses。
func TestAdaptor_GetRequestURL_ChatViaResponses(t *testing.T) {
	a := &Adaptor{}

	// responses 入站：仅勾 chat_via_responses（未勾 supports_responses）也应直连
	info := responsesUpstreamInfo(constant.RelayModeResponses, false)
	info.ChannelMeta.SupportsResponses = false
	info.ChannelMeta.ChatViaResponses = true
	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL error: %v", err)
	}
	if want := "https://upstream.example.com/v1/responses"; got != want {
		t.Errorf("responses inbound on chat_via_responses channel: URL = %q, want %q", got, want)
	}

	// chat 入站 + 桥接标志：URL 走 /v1/responses
	chatInfo := responsesUpstreamInfo(constant.RelayModeChatCompletions, false)
	chatInfo.InboundFormat = constant.RelayFormatOpenAI
	chatInfo.ClientFormat = constant.RelayFormatOpenAI
	chatInfo.ChannelMeta.SupportsResponses = false
	chatInfo.ChannelMeta.ChatViaResponses = true
	chatInfo.UseResponsesAPI = true
	got, err = a.GetRequestURL(chatInfo)
	if err != nil {
		t.Fatalf("GetRequestURL error: %v", err)
	}
	if want := "https://upstream.example.com/v1/responses"; got != want {
		t.Errorf("chat inbound via bridge: URL = %q, want %q", got, want)
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
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 5 {
		t.Errorf("cached usage = %+v, want cached=5", usage)
	}
}

// fakeResponseRouteStore 路由存储 fake，用于断言 Record 调用
type fakeResponseRouteStore struct {
	calls []fakeRouteCall
}

type fakeRouteCall struct {
	tenantID   int64
	responseID string
	route      common.ResponseRoute
}

func (f *fakeResponseRouteStore) Record(_ context.Context, tenantID int64, responseID string, route common.ResponseRoute) {
	f.calls = append(f.calls, fakeRouteCall{tenantID: tenantID, responseID: responseID, route: route})
}

func (f *fakeResponseRouteStore) Lookup(context.Context, int64, string) (common.ResponseRoute, bool) {
	return common.ResponseRoute{}, false
}

func (f *fakeResponseRouteStore) Delete(context.Context, int64, string) {}

// TestBuildResponsesObjectMap_StoreFalseEcho 流式合成 response 对象同样 store:false + echo。
func TestBuildResponsesObjectMap_StoreFalseEcho(t *testing.T) {
	temp := 0.5
	info := &common.RelayInfo{
		ResponsesRequest: &dto.OpenAIResponsesRequest{Temperature: &temp},
	}
	m := BuildResponsesObjectMap("resp_1", 1700000000, "completed", "gpt-4o", []any{}, nil, nil, info)
	if m["store"] != false {
		t.Errorf("store = %v, want false", m["store"])
	}
	if m["temperature"] != 0.5 {
		t.Errorf("temperature = %v, want 0.5", m["temperature"])
	}
}

// TestAdaptor_DoResponse_ResponsesUpstreamNonStream_RecordsRoute 直连非流式响应：
// 解析出 response id 后记录 response_id → 渠道路由（tenant 隔离、lookupModel 口径）。
func TestAdaptor_DoResponse_ResponsesUpstreamNonStream_RecordsRoute(t *testing.T) {
	fake := &fakeResponseRouteStore{}
	old := common.DefaultResponseRouteStore
	common.DefaultResponseRouteStore = fake
	t.Cleanup(func() { common.DefaultResponseRouteStore = old })

	respBody := `{"id":"resp_route1","object":"response","status":"completed","output":[]}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(respBody)),
	}
	info := responsesUpstreamInfo(constant.RelayModeResponses, false)
	info.TenantID = 42
	info.BaseModelName = "gpt-4o"
	info.ChannelMeta.ChannelID = 7

	rec := httptest.NewRecorder()
	a := &Adaptor{}
	if _, err := a.DoResponse(context.Background(), resp, info, rec); err != nil {
		t.Fatalf("DoResponse error: %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("Record calls = %d, want 1", len(fake.calls))
	}
	call := fake.calls[0]
	if call.tenantID != 42 || call.responseID != "resp_route1" {
		t.Errorf("call = %+v", call)
	}
	if call.route.ChannelID != 7 || call.route.ModelName != "gpt-4o" {
		t.Errorf("route = %+v", call.route)
	}
}

// TestAdaptor_DoResponse_ResponsesUpstreamStream_RecordsRoute 直连流式响应：
// response.created 事件即记录路由（cancel 尽早可用），completed 再刷新。
func TestAdaptor_DoResponse_ResponsesUpstreamStream_RecordsRoute(t *testing.T) {
	fake := &fakeResponseRouteStore{}
	old := common.DefaultResponseRouteStore
	common.DefaultResponseRouteStore = fake
	t.Cleanup(func() { common.DefaultResponseRouteStore = old })

	ss := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_s1","object":"response","status":"in_progress"}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_s1","status":"completed","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`,
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
	info.TenantID = 42
	info.BaseModelName = "gpt-4o"
	info.ChannelMeta.ChannelID = 7

	rec := httptest.NewRecorder()
	a := &Adaptor{}
	if _, err := a.DoResponse(context.Background(), resp, info, rec); err != nil {
		t.Fatalf("DoResponse error: %v", err)
	}
	if len(fake.calls) < 2 {
		t.Fatalf("Record calls = %d, want >=2（created + completed）", len(fake.calls))
	}
	if fake.calls[0].responseID != "resp_s1" || fake.calls[0].tenantID != 42 {
		t.Errorf("first call = %+v", fake.calls[0])
	}
	if fake.calls[len(fake.calls)-1].route.ChannelID != 7 {
		t.Errorf("last call route = %+v", fake.calls[len(fake.calls)-1].route)
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

// TestAdaptor_ConvertRequest_ChatViaResponsesBridge responses-only 上游桥接：
// chat 请求体转换为 Responses 格式（messages→input），thinking 后缀映射 reasoning.effort，
// 不注入 chat 专属 stream_options。
// claude/gemini 入站的第二跳（ConvertToOpenAI 转 chat 后）同在本分支完成，见下方回归测试。
func TestAdaptor_ConvertRequest_ChatViaResponsesBridge(t *testing.T) {
	info := responsesUpstreamInfo(constant.RelayModeChatCompletions, true)
	info.InboundFormat = constant.RelayFormatOpenAI
	info.ClientFormat = constant.RelayFormatOpenAI
	info.OriginModelName = "gpt-4o"
	info.ChannelMeta.SupportsResponses = false
	info.ChannelMeta.ChatViaResponses = true
	info.UseResponsesAPI = true

	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"say hi"}],"stream":true}`)
	a := &Adaptor{}
	out, err := a.ConvertRequest(context.Background(), info, body)
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	raw, _ := io.ReadAll(out)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("bad bridged json: %v (raw=%s)", err, raw)
	}
	if _, ok := m["messages"]; ok {
		t.Error("bridge should convert messages to responses input")
	}
	if _, ok := m["input"]; !ok {
		t.Error("bridge output missing input field")
	}
	if _, ok := m["stream_options"]; ok {
		t.Error("bridge should NOT inject chat stream_options")
	}
	if stream := string(m["stream"]); stream != "true" {
		t.Errorf("stream = %s, want true", stream)
	}
}

// TestAdaptor_ConvertRequest_ChatViaResponsesBridge_ClaudeInbound 回归（2026-08-21 收割
// 误伤修复）：claude 入站 × ChatViaResponses 渠道——handler 桥只路由 openai 入站的
// chat→responses，claude 入站经 ConvertToOpenAI 转 chat 后必须在本分支完成第二跳，
// 不得报「handler 层桥接未接管」错误。
func TestAdaptor_ConvertRequest_ChatViaResponsesBridge_ClaudeInbound(t *testing.T) {
	info := responsesUpstreamInfo(constant.RelayModeClaudeMessages, true)
	info.InboundFormat = constant.RelayFormatClaude
	info.ClientFormat = constant.RelayFormatClaude
	info.OriginModelName = "glm-4.7"
	info.ChannelMeta.SupportsResponses = false
	info.ChannelMeta.ChatViaResponses = true
	info.UseResponsesAPI = true

	body := []byte(`{"model":"glm-4.7","max_tokens":1024,"messages":[{"role":"user","content":"列出目录文件"}],"stream":true}`)
	a := &Adaptor{}
	out, err := a.ConvertRequest(context.Background(), info, body)
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	raw, _ := io.ReadAll(out)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("bad bridged json: %v (raw=%s)", err, raw)
	}
	if _, ok := m["messages"]; ok {
		t.Error("claude body should be converted to responses format (no messages key)")
	}
	if _, ok := m["input"]; !ok {
		t.Error("bridge output missing input field")
	}
	// claude 必填 max_tokens 应已吸收进 Responses 的 max_output_tokens
	if maxOut := string(m["max_output_tokens"]); maxOut != "1024" {
		t.Errorf("max_output_tokens = %s, want 1024", maxOut)
	}
}

// responsesInboundInfo responses 入站 + 上游 chat 渠道（无 responses 协议能力），
// 响应侧走 chat→responses 桥接（adaptor DoResponse 内的 relaykit 流式转换）。
func responsesInboundInfo(isStream bool) *common.RelayInfo {
	info := responsesUpstreamInfo(constant.RelayModeResponses, isStream)
	info.ChannelMeta.SupportsResponses = false
	info.ChannelMeta.ChatViaResponses = false
	return info
}

// TestAdaptor_DoResponse_ResponsesInboundStream_EmbeddedErrorBeforeEvents
// 上游 200 + SSE 但首个 data 行即内嵌错误对象（部分聚合商的出错形态）：
// 必须返回 upstream error 并向客户端透传错误体，而非静默合成空 response.completed。
func TestAdaptor_DoResponse_ResponsesInboundStream_EmbeddedErrorBeforeEvents(t *testing.T) {
	errLine := `{"error":{"type":"rate_limit_error","message":"Rate limit reached"}}`
	ss := strings.Join([]string{
		"data: " + errLine,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(ss)),
	}

	info := responsesInboundInfo(true)
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	_, err := a.DoResponse(context.Background(), resp, info, rec)
	if err == nil {
		t.Fatal("embedded upstream error should return an upstream error, got nil")
	}
	var relayErr *constant.RelayError
	if !errors.As(err, &relayErr) {
		t.Fatalf("error should be *constant.RelayError, got %T", err)
	}
	if !relayErr.ResponseWritten {
		t.Error("ResponseWritten should be true after writing error body to client")
	}
	if !strings.Contains(rec.Body.String(), "rate_limit_error") {
		t.Errorf("error body not passed through: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "response.completed") {
		t.Errorf("should not synthesize empty response.completed on upstream error: %s", rec.Body.String())
	}
}

// TestAdaptor_DoResponse_ResponsesInboundStream_EmbeddedErrorMidStream
// 流中途出现内嵌错误：已发送的事件保留，处理终止并返回 upstream error。
func TestAdaptor_DoResponse_ResponsesInboundStream_EmbeddedErrorMidStream(t *testing.T) {
	ss := strings.Join([]string{
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"finish_reason":null}]}`,
		"",
		`data: {"error":{"type":"server_error","message":"upstream crashed"}}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(ss)),
	}

	info := responsesInboundInfo(true)
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	usage, err := a.DoResponse(context.Background(), resp, info, rec)
	if err == nil {
		t.Fatal("mid-stream embedded error should return an upstream error, got nil")
	}
	if usage == nil {
		t.Fatal("usage should be non-nil for error path")
	}
	out := rec.Body.String()
	if !strings.Contains(out, "response.created") || !strings.Contains(out, `"delta":"Hi"`) {
		t.Errorf("events emitted before the error should remain:\n%s", out)
	}
	if strings.Contains(out, "response.completed") {
		t.Errorf("should not synthesize response.completed after mid-stream error:\n%s", out)
	}
}

// TestAdaptor_DoResponse_ResponsesInboundStream_UpstreamResponsesSSE
// 上游返回 Responses 格式 SSE（渠道误配为 chat-only 但上游实际说 Responses 的典型形态）：
// 每个 data 行都能解析为"空 chat chunk"，绝不能静默合成空 response.completed 假成功，
// 必须返回 502 upstream error（此时已发过前置事件，无法回写错误体）。
func TestAdaptor_DoResponse_ResponsesInboundStream_UpstreamResponsesSSE(t *testing.T) {
	ss := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1"}}`,
		"",
		`data: {"type":"response.output_text.delta","item_id":"msg_1","delta":"Hi"}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(ss)),
	}

	info := responsesInboundInfo(true)
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	usage, err := a.DoResponse(context.Background(), resp, info, rec)
	if err == nil {
		t.Fatal("non-chat upstream stream should return an upstream error, got nil")
	}
	if usage == nil {
		t.Fatal("usage should be non-nil for error path")
	}
	var relayErr *constant.RelayError
	if !errors.As(err, &relayErr) {
		t.Fatalf("error should be *constant.RelayError, got %T", err)
	}
	if relayErr.StatusCode != http.StatusBadGateway {
		t.Errorf("status code = %d, want %d", relayErr.StatusCode, http.StatusBadGateway)
	}
	out := rec.Body.String()
	// 首个"空 chunk"触发的 created 系列事件可以保留，但不得合成假成功、不得转发 Responses delta
	if strings.Contains(out, "response.completed") {
		t.Errorf("should not synthesize empty response.completed:\n%s", out)
	}
	if strings.Contains(out, `"delta":"Hi"`) {
		t.Errorf("upstream responses delta must not be forwarded as chat delta:\n%s", out)
	}
}

// TestAdaptor_DoResponse_ResponsesInboundStream_NonSSEJSONBody
// 上游 200 但返回非流式 JSON 体（无 data: 行）：应向客户端写入 502 错误并返回 upstream error，
// 而不是合成空 response.completed。
func TestAdaptor_DoResponse_ResponsesInboundStream_NonSSEJSONBody(t *testing.T) {
	body := `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"message":{"role":"assistant","content":"Hi"}}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	info := responsesInboundInfo(true)
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	_, err := a.DoResponse(context.Background(), resp, info, rec)
	if err == nil {
		t.Fatal("non-SSE upstream body should return an upstream error, got nil")
	}
	var relayErr *constant.RelayError
	if !errors.As(err, &relayErr) {
		t.Fatalf("error should be *constant.RelayError, got %T", err)
	}
	if !relayErr.ResponseWritten {
		t.Error("ResponseWritten should be true after writing error body to client")
	}
	// SetEventStreamHeaders 已提前提交 HTTP 200，状态码无法改写为 502；
	// 但错误体必须写入（客户端可见明确错误而非空 SSE 流）
	if !strings.Contains(rec.Body.String(), "upstream_protocol_mismatch") {
		t.Errorf("error body should contain protocol mismatch code: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "response.completed") {
		t.Errorf("should not synthesize empty response.completed: %s", rec.Body.String())
	}
}

// TestAdaptor_DoResponse_ResponsesInboundStream_UnparseableChunks
// 上游 data 行全部无法解析为 chat chunk（如返回了非 chat 格式）：
// 不再合成空 response.completed 假成功，必须返回 upstream error 并向客户端写入错误体。
func TestAdaptor_DoResponse_ResponsesInboundStream_UnparseableChunks(t *testing.T) {
	ss := strings.Join([]string{
		"data: {invalid json",
		"",
		"data: [DONE]",
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(ss)),
	}

	info := responsesInboundInfo(true)
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	usage, err := a.DoResponse(context.Background(), resp, info, rec)
	if err == nil {
		t.Fatal("unparseable non-chat stream should return an upstream error, got nil")
	}
	if usage == nil {
		t.Fatal("usage should be non-nil")
	}
	var relayErr *constant.RelayError
	if !errors.As(err, &relayErr) {
		t.Fatalf("error should be *constant.RelayError, got %T", err)
	}
	if !relayErr.ResponseWritten {
		t.Error("ResponseWritten should be true after writing error body to client")
	}
	if strings.Contains(rec.Body.String(), "response.completed") {
		t.Errorf("should not synthesize empty response.completed:\n%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "upstream_protocol_mismatch") {
		t.Errorf("error body should contain protocol mismatch code: %s", rec.Body.String())
	}
}
