package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

// assertContentBlockedError 断言错误为「内容安全拦截」口径：请求类错误（4xx）而非
// 上游故障（5xx），且带 ResponseWritten。
//
// 分类很关键：安全拦截由客户端提示词触发，若按 5xx 上游错误上报，正常渠道会因用户
// 发送违规内容被扣健康分乃至熔断；ResponseWritten 则保证上层不重写响应体、不换渠道重试
// （SSE 头与 200 状态码已提交，且已补齐收尾事件）。
func assertContentBlockedError(t *testing.T, err error) {
	t.Helper()
	var relayErr *constant.RelayError
	if !errors.As(err, &relayErr) {
		t.Fatalf("want *constant.RelayError, got %T: %v", err, err)
	}
	if relayErr.StatusCode >= 500 {
		t.Errorf("StatusCode = %d, want 4xx（安全拦截属客户端内容问题，不得罚渠道健康）", relayErr.StatusCode)
	}
	if !relayErr.ResponseWritten {
		t.Error("ResponseWritten = false, want true（SSE 头已提交，上层不得重写响应体）")
	}
}

// assertContentBlockedNotUpstream 断言非流式安全拦截错误为请求类（4xx）。
// 不校验 ResponseWritten：非流式在报错前未写出任何字节，上层仍应写标准错误体。
func assertContentBlockedNotUpstream(t *testing.T, err error) {
	t.Helper()
	var relayErr *constant.RelayError
	if !errors.As(err, &relayErr) {
		t.Fatalf("want *constant.RelayError, got %T: %v", err, err)
	}
	if relayErr.StatusCode >= 500 {
		t.Errorf("StatusCode = %d, want 4xx（安全拦截属客户端内容问题，不得罚渠道健康）", relayErr.StatusCode)
	}
}

// claudeInboundInfo 构造 Claude 入站 + Gemini 上游的 RelayInfo
func claudeInboundInfo(stream bool) *common.RelayInfo {
	return &common.RelayInfo{
		IsStream:        stream,
		RequestID:       "req123",
		InboundFormat:   constant.RelayFormatClaude,
		ClientFormat:    constant.RelayFormatClaude,
		OriginModelName: "gemini-3-pro",
		StartTime:       time.Now(),
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderGemini),
			BaseURL:           "https://upstream.example.com",
			UpstreamModelName: "gemini-3-pro",
		},
	}
}

// claudeInboundAdaptor 构造已 Init 的适配器与配套 RelayInfo
// （流式处理器会读 a.info 判断 Code Assist 模式，未 Init 会 panic）
func claudeInboundAdaptor(stream bool) (*Adaptor, *common.RelayInfo) {
	info := claudeInboundInfo(stream)
	a := &Adaptor{}
	a.Init(info)
	return a, info
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       readCloser(body),
	}
}

func readCloser(s string) *nopCloser {
	return &nopCloser{Reader: strings.NewReader(s)}
}

type nopCloser struct{ *strings.Reader }

func (n *nopCloser) Close() error { return nil }

// sseEvent 从记录的 SSE 输出中解析出的一个事件
type sseEvent struct {
	Event string
	Data  map[string]any
}

// parseClaudeSSE 解析 Claude 格式 SSE 输出为有序事件列表
func parseClaudeSSE(t *testing.T, raw string) []sseEvent {
	t.Helper()
	var events []sseEvent
	for frame := range strings.SplitSeq(raw, "\n\n") {
		frame = strings.TrimSpace(frame)
		if frame == "" {
			continue
		}
		var ev sseEvent
		for line := range strings.SplitSeq(frame, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				ev.Event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				payload := strings.TrimPrefix(line, "data: ")
				if payload == "" {
					t.Fatalf("SSE 事件 %q 的 data 为空，客户端会 JSON.parse 失败", ev.Event)
				}
				if err := json.Unmarshal([]byte(payload), &ev.Data); err != nil {
					t.Fatalf("解析 SSE data 失败 (event=%s): %v\ndata=%s", ev.Event, err, payload)
				}
			}
		}
		if ev.Event != "" {
			events = append(events, ev)
		}
	}
	return events
}

func eventTypes(events []sseEvent) []string {
	types := make([]string, 0, len(events))
	for _, ev := range events {
		types = append(types, ev.Event)
	}
	return types
}

// ===== 非流式 =====

// TestGeminiToClaudeResponse_PreservesPartOrder thinking/text 块按 Gemini parts 原始顺序映射，
// thoughtSignature 落到 Claude 的 signature 字段。
func TestGeminiToClaudeResponse_PreservesPartOrder(t *testing.T) {
	thought := true
	geminiResp := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{{
			Content: &dto.GeminiContent{
				Role: "model",
				Parts: []dto.GeminiPart{
					{Text: "让我想想", Thought: &thought, ThoughtSignature: "sig-abc"},
					{Text: "答案是 42"},
				},
			},
			FinishReason: "STOP",
		}},
		UsageMetadata: &dto.GeminiUsageMetadata{
			PromptTokenCount:     100,
			CandidatesTokenCount: 20,
			ThoughtsTokenCount:   5,
			TotalTokenCount:      125,
		},
	}

	got := geminiToClaudeResponse(geminiResp, claudeInboundInfo(false))

	if got.Type != "message" || got.Role != "assistant" {
		t.Errorf("消息骨架不对: type=%q role=%q", got.Type, got.Role)
	}
	if len(got.Content) != 2 {
		t.Fatalf("content 块数 = %d, want 2: %+v", len(got.Content), got.Content)
	}
	if got.Content[0].Type != "thinking" {
		t.Errorf("首块应为 thinking, got %q", got.Content[0].Type)
	}
	if got.Content[0].Thinking == nil || *got.Content[0].Thinking != "让我想想" {
		t.Errorf("thinking 内容不对: %+v", got.Content[0].Thinking)
	}
	if got.Content[0].Signature != "sig-abc" {
		t.Errorf("thoughtSignature 未搬运到 signature: %q", got.Content[0].Signature)
	}
	if got.Content[1].Type != "text" || got.Content[1].Text == nil || *got.Content[1].Text != "答案是 42" {
		t.Errorf("次块应为 text: %+v", got.Content[1])
	}
	if got.StopReason != common.ClaudeEndTurn {
		t.Errorf("stop_reason = %q, want %q", got.StopReason, common.ClaudeEndTurn)
	}
}

// TestGeminiToClaudeResponse_ToolUse functionCall 合成 tool_use ID，
// 且 stop_reason 必须从 Gemini 的 STOP 改写为 Claude 的 tool_use，否则客户端不会发起工具回合。
func TestGeminiToClaudeResponse_ToolUse(t *testing.T) {
	geminiResp := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{{
			Content: &dto.GeminiContent{
				Role: "model",
				Parts: []dto.GeminiPart{
					{FunctionCall: &dto.GeminiFunctionCall{
						FunctionName: "get_weather",
						Arguments:    map[string]any{"city": "北京"},
					}},
					{FunctionCall: &dto.GeminiFunctionCall{FunctionName: "get_time"}},
				},
			},
			FinishReason: "STOP",
		}},
	}

	got := geminiToClaudeResponse(geminiResp, claudeInboundInfo(false))

	if len(got.Content) != 2 {
		t.Fatalf("content 块数 = %d, want 2", len(got.Content))
	}
	first, second := got.Content[0], got.Content[1]
	if first.Type != "tool_use" || first.Name != "get_weather" {
		t.Errorf("首个工具块不对: %+v", first)
	}
	if first.ID == "" || first.ID == second.ID {
		t.Errorf("tool_use ID 必须非空且互不相同: %q / %q", first.ID, second.ID)
	}
	if !strings.HasPrefix(first.ID, "toolu_") {
		t.Errorf("tool_use ID 应为 Claude 风格前缀: %q", first.ID)
	}
	// Arguments 为 nil 时须回退为空对象，Claude 的 tool_use.input 不允许 null
	if second.Input == nil {
		t.Errorf("空参数应回退为空对象，got nil")
	}
	if got.StopReason != common.ClaudeToolUse {
		t.Errorf("stop_reason = %q, want %q（Gemini 带 functionCall 时仍返回 STOP，须改写）",
			got.StopReason, common.ClaudeToolUse)
	}
}

// TestGeminiToClaudeResponse_EmptyContent Claude 协议要求 content 非空
func TestGeminiToClaudeResponse_EmptyContent(t *testing.T) {
	got := geminiToClaudeResponse(&dto.GeminiChatResponse{}, claudeInboundInfo(false))
	if len(got.Content) != 1 || got.Content[0].Type != "text" {
		t.Fatalf("空响应应回退为单个空 text 块: %+v", got.Content)
	}
}

// TestClaudeUsageFromGemini_CacheAndThoughts Claude 口径：input 不含缓存、output 含思考
func TestClaudeUsageFromGemini_CacheAndThoughts(t *testing.T) {
	um := &dto.GeminiUsageMetadata{
		PromptTokenCount:        100, // Gemini 语义已含 cached
		CachedContentTokenCount: 30,
		CandidatesTokenCount:    20, // Gemini 语义不含 thoughts
		ThoughtsTokenCount:      7,
	}

	got := claudeUsageFromGemini(um)

	if got.InputTokens != 70 {
		t.Errorf("input_tokens = %d, want 70（须扣除 cached）", got.InputTokens)
	}
	if got.CacheReadInputTokens != 30 {
		t.Errorf("cache_read_input_tokens = %d, want 30", got.CacheReadInputTokens)
	}
	if got.OutputTokens != 27 {
		t.Errorf("output_tokens = %d, want 27（candidates+thoughts）", got.OutputTokens)
	}
}

// TestClaudeInputTokens_NegativeGuard cached 超过 prompt 时不得出现负数
func TestClaudeInputTokens_NegativeGuard(t *testing.T) {
	got := claudeInputTokens(&dto.GeminiUsageMetadata{PromptTokenCount: 10, CachedContentTokenCount: 40})
	if got != 0 {
		t.Errorf("input_tokens = %d, want 0", got)
	}
	if claudeInputTokens(nil) != 0 {
		t.Errorf("nil usage 应返回 0")
	}
}

// TestGeminiPartFallbackText Claude 无对应块类型的 part 渲染为文本而非丢弃
func TestGeminiPartFallbackText(t *testing.T) {
	cases := []struct {
		name string
		part dto.GeminiPart
		want string
	}{
		{"inlineData", dto.GeminiPart{InlineData: &dto.GeminiInlineData{MimeType: "image/png", Data: "AAA"}},
			"![image](data:image/png;base64,AAA)"},
		{"fileData", dto.GeminiPart{FileData: &dto.GeminiFileData{FileURI: "gs://b/o"}}, "[file](gs://b/o)"},
		{"executableCode", dto.GeminiPart{ExecutableCode: &dto.GeminiExecutableCode{Language: "python", Code: "print(1)"}},
			"```python\nprint(1)\n```"},
		{"codeExecutionResult", dto.GeminiPart{CodeExecutionResult: &dto.GeminiCodeExecutionResult{Outcome: "OUTCOME_OK", Output: "1"}},
			"Execution OUTCOME_OK:\n1"},
		{"纯文本无回退", dto.GeminiPart{Text: "hi"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := geminiPartFallbackText(&tc.part); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestHandleNonStreamToClaude_WritesClaudeBody 正常路径：写出的响应体为 Claude Messages 结构，
// 而计费用量仍为 Gemini 口径（两套口径不能混）。
func TestHandleNonStreamToClaude_WritesClaudeBody(t *testing.T) {
	body := `{"candidates":[{"content":{"role":"model","parts":[{"text":"你好"}]},"finishReason":"STOP"}],` +
		`"usageMetadata":{"promptTokenCount":50,"cachedContentTokenCount":10,"candidatesTokenCount":8,"thoughtsTokenCount":3,"totalTokenCount":61}}`

	a, info := claudeInboundAdaptor(false)
	rec := httptest.NewRecorder()

	usage, err := a.handleNonStreamToClaude(context.Background(), jsonResponse(http.StatusOK, body), info, rec)
	if err != nil {
		t.Fatalf("handleNonStreamToClaude error: %v", err)
	}

	var got dto.ClaudeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应体不是合法 Claude JSON: %v\nbody=%s", err, rec.Body.String())
	}
	if got.Type != "message" || got.Role != "assistant" {
		t.Errorf("消息骨架不对: %+v", got)
	}
	if len(got.Content) != 1 || got.Content[0].Type != "text" || *got.Content[0].Text != "你好" {
		t.Errorf("content 不对: %+v", got.Content)
	}
	if got.StopReason != common.ClaudeEndTurn {
		t.Errorf("stop_reason = %q", got.StopReason)
	}
	// 客户端可见用量为 Claude 口径
	if got.Usage.InputTokens != 40 || got.Usage.OutputTokens != 11 || got.Usage.CacheReadInputTokens != 10 {
		t.Errorf("客户端用量口径不对: %+v", got.Usage)
	}
	// 计费用量为 Gemini 口径（prompt 含 cached）
	if usage.PromptTokens != 50 || usage.CompletionTokens != 11 || !usage.CacheIncludedInPrompt {
		t.Errorf("计费用量口径不对: %+v", usage)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// TestHandleNonStreamToClaude_SafetyBlock 安全过滤命中时返回请求类错误且不写响应体
func TestHandleNonStreamToClaude_SafetyBlock(t *testing.T) {
	a, info := claudeInboundAdaptor(false)
	rec := httptest.NewRecorder()
	body := `{"promptFeedback":{"blockReason":"SAFETY"}}`

	_, err := a.handleNonStreamToClaude(context.Background(), jsonResponse(http.StatusOK, body), info, rec)
	if err == nil {
		t.Fatal("安全过滤应返回错误")
	}
	// 非流式尚未写出字节，故不置 ResponseWritten；但分类同流式：4xx 而非上游 5xx
	assertContentBlockedNotUpstream(t, err)
	if rec.Body.Len() != 0 {
		t.Errorf("不应写响应体, got %q", rec.Body.String())
	}
}

// TestHandleNonStreamToClaude_InvalidBody 上游返回非法 JSON 时报错而非写出半成品
func TestHandleNonStreamToClaude_InvalidBody(t *testing.T) {
	a, info := claudeInboundAdaptor(false)
	rec := httptest.NewRecorder()

	_, err := a.handleNonStreamToClaude(context.Background(), jsonResponse(http.StatusOK, "not-json"), info, rec)
	if err == nil {
		t.Fatal("非法响应体应返回错误")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("不应写响应体, got %q", rec.Body.String())
	}
}

// TestHandleNonStreamToClaude_UpstreamErrorNotWritten 上游错误不在本层写响应，
// 交上层 WriteClaudeRelayError 统一写入（避免双写与重试时污染响应）。
func TestHandleNonStreamToClaude_UpstreamErrorNotWritten(t *testing.T) {
	a, info := claudeInboundAdaptor(false)
	rec := httptest.NewRecorder()
	body := `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","message":"quota"}}`

	_, err := a.handleNonStreamToClaude(context.Background(), jsonResponse(http.StatusTooManyRequests, body), info, rec)

	if err == nil {
		t.Fatal("上游 429 应返回错误")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("本层不应写响应体, got %q", rec.Body.String())
	}
}

// ===== 流式 =====

// TestHandleStreamToClaude_EventSequence 完整事件序列：
// message_start → content_block_start/delta/stop（thinking 与 text 各成块）→ message_delta → message_stop
func TestHandleStreamToClaude_EventSequence(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"思考中","thought":true,"thoughtSignature":"sig1"}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"你好"}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"世界"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":50,"cachedContentTokenCount":10,"candidatesTokenCount":8,"thoughtsTokenCount":3,"totalTokenCount":61}}`,
		``,
		``,
	}, "\n")

	a, info := claudeInboundAdaptor(true)
	rec := httptest.NewRecorder()

	usage, err := a.handleStreamToClaude(context.Background(), sseResponse(strings.NewReader(upstream)), info, rec)
	if err != nil {
		t.Fatalf("handleStreamToClaude error: %v", err)
	}

	events := parseClaudeSSE(t, rec.Body.String())
	got := eventTypes(events)
	want := []string{
		"message_start",
		"content_block_start", "content_block_delta", "content_block_delta", // thinking 块 + 文本增量 + signature 增量
		"content_block_stop",
		"content_block_start", "content_block_delta", "content_block_delta", // text 块 + 两次增量
		"content_block_stop",
		"message_delta", "message_stop",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("事件序列不符\ngot:  %v\nwant: %v", got, want)
	}

	// thinking 块与 signature
	if events[1].Data["content_block"].(map[string]any)["type"] != "thinking" {
		t.Errorf("首块应为 thinking: %+v", events[1].Data["content_block"])
	}
	if events[3].Data["delta"].(map[string]any)["type"] != "signature_delta" {
		t.Errorf("应发出 signature_delta: %+v", events[3].Data["delta"])
	}

	// 块索引必须连续且成对：thinking=0, text=1
	if idx := events[4].Data["index"].(float64); idx != 0 {
		t.Errorf("thinking 块 content_block_stop index = %v, want 0", idx)
	}
	if idx := events[5].Data["index"].(float64); idx != 1 {
		t.Errorf("text 块 content_block_start index = %v, want 1", idx)
	}

	// message_delta 的客户端可见用量为 Claude 口径
	delta := events[9].Data
	if delta["delta"].(map[string]any)["stop_reason"] != common.ClaudeEndTurn {
		t.Errorf("stop_reason = %v, want %q", delta["delta"], common.ClaudeEndTurn)
	}
	clientUsage := delta["usage"].(map[string]any)
	if clientUsage["input_tokens"].(float64) != 40 {
		t.Errorf("客户端 input_tokens = %v, want 40（50-10 cached）", clientUsage["input_tokens"])
	}
	if clientUsage["output_tokens"].(float64) != 11 {
		t.Errorf("客户端 output_tokens = %v, want 11（8+3 thoughts）", clientUsage["output_tokens"])
	}

	// 计费用量走 Gemini 口径（completion 含 thoughts，prompt 含 cached）
	if usage.PromptTokens != 50 || usage.CompletionTokens != 11 {
		t.Errorf("计费用量 = prompt %d / completion %d, want 50 / 11", usage.PromptTokens, usage.CompletionTokens)
	}
	if !usage.CacheIncludedInPrompt {
		t.Error("CacheIncludedInPrompt 应为 true，否则缓存部分会按 input 全价重复计费")
	}
}

// TestHandleStreamToClaude_ToolCall functionCall 转为独立 tool_use 块，
// 参数以 input_json_delta 一次性给出，stop_reason 改写为 tool_use。
func TestHandleStreamToClaude_ToolCall(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"city":"北京"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":4,"totalTokenCount":14}}`,
		``,
		``,
	}, "\n")

	a, info := claudeInboundAdaptor(true)
	rec := httptest.NewRecorder()

	_, err := a.handleStreamToClaude(context.Background(), sseResponse(strings.NewReader(upstream)), info, rec)
	if err != nil {
		t.Fatalf("handleStreamToClaude error: %v", err)
	}

	events := parseClaudeSSE(t, rec.Body.String())
	got := eventTypes(events)
	want := []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("事件序列不符\ngot:  %v\nwant: %v", got, want)
	}

	block := events[1].Data["content_block"].(map[string]any)
	if block["type"] != "tool_use" || block["name"] != "get_weather" {
		t.Errorf("tool_use 块不对: %+v", block)
	}
	if id, _ := block["id"].(string); !strings.HasPrefix(id, "toolu_") {
		t.Errorf("tool_use id = %q, want toolu_ 前缀", id)
	}

	delta := events[2].Data["delta"].(map[string]any)
	if delta["type"] != "input_json_delta" {
		t.Fatalf("参数应以 input_json_delta 给出: %+v", delta)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(delta["partial_json"].(string)), &args); err != nil {
		t.Fatalf("partial_json 不是合法 JSON: %v", err)
	}
	if args["city"] != "北京" {
		t.Errorf("参数丢失: %+v", args)
	}

	if events[4].Data["delta"].(map[string]any)["stop_reason"] != common.ClaudeToolUse {
		t.Errorf("stop_reason 应改写为 tool_use: %+v", events[4].Data["delta"])
	}
}

// TestHandleStreamToClaude_EmptyUpstreamStillClosesSequence 上游无有效 chunk 时，
// 仍须补齐 message_start/message_stop，否则 Anthropic 客户端会一直等收尾事件。
func TestHandleStreamToClaude_EmptyUpstreamStillClosesSequence(t *testing.T) {
	a, info := claudeInboundAdaptor(true)
	rec := httptest.NewRecorder()

	_, err := a.handleStreamToClaude(context.Background(), sseResponse(strings.NewReader("\n\n")), info, rec)
	if err != nil {
		t.Fatalf("handleStreamToClaude error: %v", err)
	}

	got := eventTypes(parseClaudeSSE(t, rec.Body.String()))
	want := []string{"message_start", "message_delta", "message_stop"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("事件序列不符\ngot:  %v\nwant: %v", got, want)
	}
}

// TestHandleStreamToClaude_UpstreamErrorNotWritten 流式上游错误同样不在本层写响应
func TestHandleStreamToClaude_UpstreamErrorNotWritten(t *testing.T) {
	a, info := claudeInboundAdaptor(true)
	rec := httptest.NewRecorder()
	resp := jsonResponse(http.StatusServiceUnavailable, `{"error":{"code":503,"status":"UNAVAILABLE","message":"down"}}`)

	_, err := a.handleStreamToClaude(context.Background(), resp, info, rec)

	if err == nil {
		t.Fatal("上游 503 应返回错误")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("本层不应写响应体, got %q", rec.Body.String())
	}
}

// TestHandleStreamToClaude_SafetyBlockClosesSequence 安全过滤命中时 SSE 头已发出，
// 必须补齐收尾事件（否则客户端挂起）并向上返回错误。
// 安全拦截属客户端内容问题：错误须为请求类（4xx），不得按上游故障罚渠道健康；
// 且带 ResponseWritten（SSE 头已提交，上层不得重写响应体或换渠道重试）。
func TestHandleStreamToClaude_SafetyBlockClosesSequence(t *testing.T) {
	upstream := "data: {\"promptFeedback\":{\"blockReason\":\"SAFETY\"}}\n\n"

	a, info := claudeInboundAdaptor(true)
	rec := httptest.NewRecorder()

	_, err := a.handleStreamToClaude(context.Background(), sseResponse(strings.NewReader(upstream)), info, rec)
	if err == nil {
		t.Fatal("安全过滤应返回错误")
	}
	assertContentBlockedError(t, err)
	if got := info.StreamStatus.GetEndReason(); got != common.StreamEndReasonError {
		t.Errorf("StreamStatus end reason = %v, want %v", got, common.StreamEndReasonError)
	}

	got := eventTypes(parseClaudeSSE(t, rec.Body.String()))
	if len(got) == 0 || got[len(got)-1] != "message_stop" {
		t.Errorf("必须以 message_stop 收尾, got %v", got)
	}
}
