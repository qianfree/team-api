package claude_gemini

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// claudeInboundMeta 构造 Claude 入站 + Gemini 上游的 Meta
func claudeInboundMeta(isStream bool) *convmeta.Values {
	return &convmeta.Values{
		OriginModelName:     "gemini-3-pro",
		UpstreamModelName:   "gemini-3-pro",
		ChannelMetaAttached: true,
		IsStream:            isStream,
	}
}

// runGeminiToClaudeStream 跑流式转换并收集 StreamEvent 序列
func runGeminiToClaudeStream(t *testing.T, meta convmeta.Meta, upstream string) ([]*relayconvert.StreamEvent, error) {
	t.Helper()
	conv := &GeminiToClaudeStreamConverter{}
	var events []*relayconvert.StreamEvent
	err := conv.ConvertStreamResponse(context.Background(), meta, strings.NewReader(upstream), func(chunk any) error {
		ev, ok := chunk.(*relayconvert.StreamEvent)
		if !ok {
			t.Fatalf("chunk 类型应为 *relayconvert.StreamEvent, got %T", chunk)
		}
		events = append(events, ev)
		return nil
	})
	return events, err
}

// claudeEventTypes 提取事件名序列（校验帧形态：Claude 客户端为事件帧）
func claudeEventTypes(t *testing.T, events []*relayconvert.StreamEvent) []string {
	t.Helper()
	names := make([]string, 0, len(events))
	for _, ev := range events {
		if ev.Event == "" {
			t.Fatalf("Claude 客户端应为事件帧（Event 非空）: %+v", ev)
		}
		names = append(names, ev.Event)
	}
	return names
}

// claudeDataOf 取事件负载并断言类型
func claudeDataOf(t *testing.T, ev *relayconvert.StreamEvent) *dto.ClaudeResponse {
	t.Helper()
	data, ok := ev.Data.(*dto.ClaudeResponse)
	if !ok {
		t.Fatalf("data 类型应为 *dto.ClaudeResponse, got %T", ev.Data)
	}
	return data
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

	conv := &GeminiToClaudeResponseConverter{}
	res, err := conv.ConvertResponse(context.Background(), claudeInboundMeta(false), geminiResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	got, ok := res.(*dto.ClaudeResponse)
	if !ok {
		t.Fatalf("返回类型应为 *dto.ClaudeResponse, got %T", res)
	}

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
	if got.StopReason != claudeEndTurn {
		t.Errorf("stop_reason = %q, want %q", got.StopReason, claudeEndTurn)
	}
	// 客户端可见用量为 Claude 口径（input 不含缓存，output 含思考）
	if got.Usage.InputTokens != 100 || got.Usage.OutputTokens != 25 {
		t.Errorf("客户端用量口径不对: %+v", got.Usage)
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

	conv := &GeminiToClaudeResponseConverter{}
	res, err := conv.ConvertResponse(context.Background(), claudeInboundMeta(false), geminiResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	got := res.(*dto.ClaudeResponse)

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
	if got.StopReason != claudeToolUse {
		t.Errorf("stop_reason = %q, want %q（Gemini 带 functionCall 时仍返回 STOP，须改写）",
			got.StopReason, claudeToolUse)
	}
}

// TestGeminiToClaudeResponse_EmptyContent Claude 协议要求 content 非空
func TestGeminiToClaudeResponse_EmptyContent(t *testing.T) {
	conv := &GeminiToClaudeResponseConverter{}
	res, err := conv.ConvertResponse(context.Background(), claudeInboundMeta(false), &dto.GeminiChatResponse{})
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	got := res.(*dto.ClaudeResponse)
	if len(got.Content) != 1 || got.Content[0].Type != "text" {
		t.Fatalf("空响应应回退为单个空 text 块: %+v", got.Content)
	}
}

// TestGeminiToClaudeResponse_SafetyBlock 安全过滤命中时返回错误而非合成响应
func TestGeminiToClaudeResponse_SafetyBlock(t *testing.T) {
	conv := &GeminiToClaudeResponseConverter{}
	_, err := conv.ConvertResponse(context.Background(), claudeInboundMeta(false), &dto.GeminiChatResponse{
		PromptFeedback: &dto.GeminiPromptFeedback{BlockReason: "SAFETY"},
	})
	if err == nil {
		t.Fatal("安全过滤应返回错误")
	}
	if !strings.Contains(err.Error(), "SAFETY") {
		t.Errorf("错误信息应包含 blockReason: %v", err)
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

// ===== 流式 =====

// TestGeminiToClaudeStream_EventSequence 完整事件序列：
// message_start → content_block_start/delta/stop（thinking 与 text 各成块）→ message_delta → message_stop
func TestGeminiToClaudeStream_EventSequence(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"思考中","thought":true,"thoughtSignature":"sig1"}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"你好"}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"世界"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":50,"cachedContentTokenCount":10,"candidatesTokenCount":8,"thoughtsTokenCount":3,"totalTokenCount":61}}`,
		``,
		``,
	}, "\n")

	events, err := runGeminiToClaudeStream(t, claudeInboundMeta(true), upstream)
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	got := claudeEventTypes(t, events)
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
	if claudeDataOf(t, events[1]).ContentBlock.Type != "thinking" {
		t.Errorf("首块应为 thinking: %+v", claudeDataOf(t, events[1]).ContentBlock)
	}
	if claudeDataOf(t, events[3]).Delta.Type != "signature_delta" {
		t.Errorf("应发出 signature_delta: %+v", claudeDataOf(t, events[3]).Delta)
	}
	if claudeDataOf(t, events[3]).Delta.Signature != "sig1" {
		t.Errorf("signature 内容不对: %+v", claudeDataOf(t, events[3]).Delta)
	}

	// 块索引必须连续且成对：thinking=0, text=1
	if idx := claudeDataOf(t, events[4]).Index; idx == nil || *idx != 0 {
		t.Errorf("thinking 块 content_block_stop index = %v, want 0", idx)
	}
	if idx := claudeDataOf(t, events[5]).Index; idx == nil || *idx != 1 {
		t.Errorf("text 块 content_block_start index = %v, want 1", idx)
	}

	// message_delta 的客户端可见用量为 Claude 口径
	delta := claudeDataOf(t, events[9])
	if delta.Delta.StopReason == nil || *delta.Delta.StopReason != claudeEndTurn {
		t.Errorf("stop_reason = %v, want %q", delta.Delta.StopReason, claudeEndTurn)
	}
	if delta.Usage.InputTokens != 40 {
		t.Errorf("客户端 input_tokens = %v, want 40（50-10 cached）", delta.Usage.InputTokens)
	}
	if delta.Usage.OutputTokens != 11 {
		t.Errorf("客户端 output_tokens = %v, want 11（8+3 thoughts）", delta.Usage.OutputTokens)
	}

	// 计费用量走 Gemini 口径（completion 含 thoughts，prompt 含 cached），随 message_delta 帧携带
	if events[9].Usage == nil {
		t.Fatal("message_delta 帧必须携带计费用量")
	}
	if events[9].Usage.PromptTokens != 50 || events[9].Usage.CompletionTokens != 11 {
		t.Errorf("计费用量 = prompt %d / completion %d, want 50 / 11",
			events[9].Usage.PromptTokens, events[9].Usage.CompletionTokens)
	}
	if events[9].Usage.PromptTokensDetails == nil || events[9].Usage.PromptTokensDetails.CachedTokens != 10 {
		t.Errorf("计费缓存明细不对: %+v", events[9].Usage.PromptTokensDetails)
	}
	if events[9].Usage.CompletionTokenDetails == nil || events[9].Usage.CompletionTokenDetails.ReasoningTokens != 3 {
		t.Errorf("计费思考明细不对: %+v", events[9].Usage.CompletionTokenDetails)
	}
}

// TestGeminiToClaudeStream_ToolCall functionCall 转为独立 tool_use 块，
// 参数以 input_json_delta 一次性给出，stop_reason 改写为 tool_use。
func TestGeminiToClaudeStream_ToolCall(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"city":"北京"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":4,"totalTokenCount":14}}`,
		``,
		``,
	}, "\n")

	events, err := runGeminiToClaudeStream(t, claudeInboundMeta(true), upstream)
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	got := claudeEventTypes(t, events)
	want := []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("事件序列不符\ngot:  %v\nwant: %v", got, want)
	}

	block := claudeDataOf(t, events[1]).ContentBlock
	if block.Type != "tool_use" || block.Name != "get_weather" {
		t.Errorf("tool_use 块不对: %+v", block)
	}
	if !strings.HasPrefix(block.ID, "toolu_") {
		t.Errorf("tool_use id = %q, want toolu_ 前缀", block.ID)
	}

	delta := claudeDataOf(t, events[2]).Delta
	if delta.Type != "input_json_delta" {
		t.Fatalf("参数应以 input_json_delta 给出: %+v", delta)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(*delta.PartialJSON), &args); err != nil {
		t.Fatalf("partial_json 不是合法 JSON: %v", err)
	}
	if args["city"] != "北京" {
		t.Errorf("参数丢失: %+v", args)
	}

	final := claudeDataOf(t, events[4]).Delta
	if final.StopReason == nil || *final.StopReason != claudeToolUse {
		t.Errorf("stop_reason 应改写为 tool_use: %+v", final)
	}
}

// TestGeminiToClaudeStream_EmptyUpstreamStillClosesSequence 上游无有效 chunk 时，
// 仍须补齐 message_start/message_stop，否则 Anthropic 客户端会一直等收尾事件。
func TestGeminiToClaudeStream_EmptyUpstreamStillClosesSequence(t *testing.T) {
	events, err := runGeminiToClaudeStream(t, claudeInboundMeta(true), "\n\n")
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	got := claudeEventTypes(t, events)
	want := []string{"message_start", "message_delta", "message_stop"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("事件序列不符\ngot:  %v\nwant: %v", got, want)
	}
}

// TestGeminiToClaudeStream_SafetyBlockClosesSequence 安全过滤命中时事件流已开场，
// 必须补齐收尾事件再返回错误，否则客户端挂起。
func TestGeminiToClaudeStream_SafetyBlockClosesSequence(t *testing.T) {
	upstream := "data: {\"promptFeedback\":{\"blockReason\":\"SAFETY\"}}\n\n"

	events, err := runGeminiToClaudeStream(t, claudeInboundMeta(true), upstream)
	if err == nil {
		t.Fatal("安全过滤应返回错误")
	}

	got := claudeEventTypes(t, events)
	if len(got) == 0 || got[len(got)-1] != "message_stop" {
		t.Errorf("必须以 message_stop 收尾, got %v", got)
	}
	// 与旧桥一致：安全过滤路径不产出计费用量
	for _, ev := range events {
		if ev.Usage != nil {
			t.Errorf("安全过滤路径不应携带计费用量: %+v", ev)
		}
	}
}
