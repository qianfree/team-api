package claude_gemini

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// geminiInboundMeta 构造 Gemini 入站 + Claude 上游渠道的 Meta
func geminiInboundMeta(isStream bool) *convmeta.Values {
	return &convmeta.Values{
		OriginModelName:     "claude-sonnet-5",
		UpstreamModelName:   "claude-sonnet-5",
		ChannelMetaAttached: true,
		IsStream:            isStream,
	}
}

// runClaudeToGeminiStream 跑流式转换并收集 StreamEvent 序列
func runClaudeToGeminiStream(t *testing.T, meta convmeta.Meta, upstream string) ([]*relayconvert.StreamEvent, error) {
	t.Helper()
	conv := &ClaudeToGeminiStreamConverter{}
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

// geminiChunksOf 提取事件序列中的 Gemini chunk（校验帧形态：Gemini 客户端为纯 data 帧）
func geminiChunksOf(t *testing.T, events []*relayconvert.StreamEvent) []*dto.GeminiChatResponse {
	t.Helper()
	chunks := make([]*dto.GeminiChatResponse, 0, len(events))
	for _, ev := range events {
		if ev.Event != "" {
			t.Fatalf("Gemini 客户端应为纯 data 帧（Event 为空）, got %q", ev.Event)
		}
		chunk, ok := ev.Data.(*dto.GeminiChatResponse)
		if !ok {
			t.Fatalf("data 类型应为 *dto.GeminiChatResponse, got %T", ev.Data)
		}
		chunks = append(chunks, chunk)
	}
	return chunks
}

// collectParts 把所有 chunk 的 parts 按顺序摊平
func collectParts(chunks []*dto.GeminiChatResponse) []dto.GeminiPart {
	var parts []dto.GeminiPart
	for _, c := range chunks {
		for _, cand := range c.Candidates {
			if cand.Content != nil {
				parts = append(parts, cand.Content.Parts...)
			}
		}
	}
	return parts
}

// ===== 非流式 =====

// TestClaudeToGeminiResponse_BlockMapping text/thinking/tool_use 按原始顺序映射为 parts，
// redacted_thinking 无对应物被丢弃。
func TestClaudeToGeminiResponse_BlockMapping(t *testing.T) {
	thinking := "让我想想"
	text := "答案是 42"
	claudeResp := &dto.ClaudeResponse{
		ID:    "msg_1",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-sonnet-5",
		Content: []dto.ClaudeContentBlock{
			{Type: "thinking", Thinking: &thinking, Signature: "sig-abc"},
			{Type: "redacted_thinking"},
			{Type: "text", Text: &text},
			{Type: "tool_use", ID: "toolu_9", Name: "get_weather", Input: map[string]any{"city": "北京"}},
		},
		StopReason: claudeToolUse,
		Usage: &dto.ClaudeUsage{
			InputTokens:              40,
			CacheReadInputTokens:     10,
			CacheCreationInputTokens: 5,
			OutputTokens:             20,
		},
	}

	conv := &ClaudeToGeminiResponseConverter{}
	res, err := conv.ConvertResponse(context.Background(), geminiInboundMeta(false), claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	got, ok := res.(*dto.GeminiChatResponse)
	if !ok {
		t.Fatalf("返回类型应为 *dto.GeminiChatResponse, got %T", res)
	}

	if len(got.Candidates) != 1 || got.Candidates[0].Content == nil {
		t.Fatalf("candidates 结构不对: %+v", got.Candidates)
	}
	if got.Candidates[0].Content.Role != "model" {
		t.Errorf("role = %q, want model", got.Candidates[0].Content.Role)
	}
	parts := got.Candidates[0].Content.Parts
	if len(parts) != 3 {
		t.Fatalf("parts 数 = %d, want 3（redacted_thinking 应被丢弃）: %+v", len(parts), parts)
	}

	// thinking → thought part + thoughtSignature
	if parts[0].Text != thinking {
		t.Errorf("首个 part 应为思考文本: %+v", parts[0])
	}
	if parts[0].Thought == nil || !*parts[0].Thought {
		t.Errorf("思考 part 必须带 thought=true: %+v", parts[0])
	}
	if parts[0].ThoughtSignature != "sig-abc" {
		t.Errorf("signature 未搬运到 thoughtSignature: %q", parts[0].ThoughtSignature)
	}

	// text
	if parts[1].Text != text || parts[1].Thought != nil {
		t.Errorf("次个 part 应为普通文本: %+v", parts[1])
	}

	// tool_use → functionCall（id 可搬运，Gemini 的 functionCall 支持可选 id）
	if parts[2].FunctionCall == nil {
		t.Fatalf("第三个 part 应为 functionCall: %+v", parts[2])
	}
	fc := parts[2].FunctionCall
	if fc.FunctionName != "get_weather" || fc.ID != "toolu_9" {
		t.Errorf("functionCall 不对: %+v", fc)
	}

	if got.Candidates[0].FinishReason != geminiSTOP {
		t.Errorf("finishReason = %q, want STOP", got.Candidates[0].FinishReason)
	}

	if got.ModelName != "claude-sonnet-5" {
		t.Errorf("modelName = %q, want claude-sonnet-5", got.ModelName)
	}

	// Gemini 口径：prompt 含全部缓存
	if got.UsageMetadata.PromptTokenCount != 55 {
		t.Errorf("promptTokenCount = %d, want 55（40+10+5）", got.UsageMetadata.PromptTokenCount)
	}
	if got.UsageMetadata.CachedContentTokenCount != 10 {
		t.Errorf("cachedContentTokenCount = %d, want 10", got.UsageMetadata.CachedContentTokenCount)
	}
	if got.UsageMetadata.CandidatesTokenCount != 20 || got.UsageMetadata.ThoughtsTokenCount != 0 {
		t.Errorf("Claude 不拆分思考 token，应全部计入 candidates: %+v", got.UsageMetadata)
	}
	if got.UsageMetadata.TotalTokenCount != 75 {
		t.Errorf("totalTokenCount = %d, want 75", got.UsageMetadata.TotalTokenCount)
	}
}

// TestClaudeToGeminiResponse_NullToolInput Claude 允许 input 为 null，
// Gemini 的 functionCall.args 期望对象。
func TestClaudeToGeminiResponse_NullToolInput(t *testing.T) {
	claudeResp := &dto.ClaudeResponse{
		Content:    []dto.ClaudeContentBlock{{Type: "tool_use", ID: "t1", Name: "ping"}},
		StopReason: claudeToolUse,
	}

	conv := &ClaudeToGeminiResponseConverter{}
	res, err := conv.ConvertResponse(context.Background(), geminiInboundMeta(false), claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse error: %v", err)
	}
	got := res.(*dto.GeminiChatResponse)

	fc := got.Candidates[0].Content.Parts[0].FunctionCall
	if fc.Arguments == nil {
		t.Fatal("空 input 应回退为空对象，got nil")
	}
	if _, ok := fc.Arguments.(map[string]any); !ok {
		t.Errorf("args 应为对象: %T", fc.Arguments)
	}
}

// TestClaudeToGeminiResponse_WrongType 类型不匹配时报错而非 panic
func TestClaudeToGeminiResponse_WrongType(t *testing.T) {
	conv := &ClaudeToGeminiResponseConverter{}
	if _, err := conv.ConvertResponse(context.Background(), geminiInboundMeta(false), "not-a-response"); err == nil {
		t.Fatal("非 *dto.ClaudeResponse 输入应返回错误")
	}
}

// TestGeminiUsageFromClaude_NilAndEmpty 无用量信息时不应造出全零的 usageMetadata
func TestGeminiUsageFromClaude_NilAndEmpty(t *testing.T) {
	if geminiUsageFromClaude(nil) != nil {
		t.Error("nil usage 应返回 nil")
	}
	if geminiUsageFromClaude(&dto.ClaudeUsage{}) != nil {
		t.Error("全零 usage 应返回 nil，避免向客户端报 0 token")
	}
}

// ===== 流式 =====

// TestClaudeToGeminiStream_TextAndThinking 思考与正文分别映射为 thought part / 普通 part，
// 收尾 chunk 带 finishReason 与 usageMetadata，并随帧携带 Claude 计费口径的用量。
func TestClaudeToGeminiStream_TextAndThinking(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5","content":[],"usage":{"input_tokens":40,"output_tokens":0,"cache_read_input_tokens":10}}}`,
		``,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"想一下"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig-xyz"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		``,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"你好"}}`,
		``,
		`data: {"type":"content_block_stop","index":1}`,
		``,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":40,"output_tokens":20,"cache_read_input_tokens":10}}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
		``,
	}, "\n")

	events, err := runClaudeToGeminiStream(t, geminiInboundMeta(true), upstream)
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	chunks := geminiChunksOf(t, events)
	if len(chunks) < 2 {
		t.Fatalf("chunk 数过少: %d", len(chunks))
	}

	parts := collectParts(chunks)
	if len(parts) != 3 {
		t.Fatalf("parts 数 = %d, want 3（thinking / signature / text）: %+v", len(parts), parts)
	}
	if parts[0].Text != "想一下" || parts[0].Thought == nil || !*parts[0].Thought {
		t.Errorf("思考 part 不对: %+v", parts[0])
	}
	if parts[1].ThoughtSignature != "sig-xyz" {
		t.Errorf("signature part 不对: %+v", parts[1])
	}
	if parts[2].Text != "你好" || parts[2].Thought != nil {
		t.Errorf("文本 part 不对: %+v", parts[2])
	}

	final := chunks[len(chunks)-1]
	if final.Candidates[0].FinishReason != geminiSTOP {
		t.Errorf("收尾 finishReason = %q, want STOP", final.Candidates[0].FinishReason)
	}
	if final.UsageMetadata == nil || final.UsageMetadata.PromptTokenCount != 50 {
		t.Errorf("收尾 usageMetadata 不对（应为 40+10 缓存）: %+v", final.UsageMetadata)
	}
	if final.ModelName != "claude-sonnet-5" {
		t.Errorf("modelName = %q, want claude-sonnet-5", final.ModelName)
	}

	// 计费口径：Claude 的 input_tokens 不含缓存，随收尾帧携带
	lastEvent := events[len(events)-1]
	if lastEvent.Usage == nil {
		t.Fatal("收尾帧必须携带计费用量")
	}
	if lastEvent.Usage.PromptTokens != 40 || lastEvent.Usage.CompletionTokens != 20 {
		t.Errorf("计费用量 = %d/%d, want 40/20", lastEvent.Usage.PromptTokens, lastEvent.Usage.CompletionTokens)
	}
	if lastEvent.Usage.PromptTokensDetails == nil || lastEvent.Usage.PromptTokensDetails.CachedTokens != 10 {
		t.Errorf("计费缓存明细不对: %+v", lastEvent.Usage.PromptTokensDetails)
	}
	// 前置内容帧不带用量
	for _, ev := range events[:len(events)-1] {
		if ev.Usage != nil {
			t.Errorf("非收尾帧不应携带用量: %+v", ev)
		}
	}
}

// TestClaudeToGeminiStream_ToolCallBuffered Claude 的 input_json_delta 分片必须
// 缓冲拼接成完整对象后才能作为 functionCall.args 发出（Gemini 不接受分片）。
func TestClaudeToGeminiStream_ToolCallBuffered(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5","content":[],"usage":{"input_tokens":10,"output_tokens":0}}}`,
		``,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_7","name":"get_weather","input":{}}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"北京\"}"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":10,"output_tokens":6}}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
		``,
	}, "\n")

	events, err := runClaudeToGeminiStream(t, geminiInboundMeta(true), upstream)
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	parts := collectParts(geminiChunksOf(t, events))
	if len(parts) != 1 || parts[0].FunctionCall == nil {
		t.Fatalf("应恰好产出一个 functionCall part: %+v", parts)
	}

	fc := parts[0].FunctionCall
	if fc.FunctionName != "get_weather" || fc.ID != "toolu_7" {
		t.Errorf("functionCall 元信息不对: %+v", fc)
	}
	args, ok := fc.Arguments.(map[string]any)
	if !ok {
		t.Fatalf("args 应为拼接后的对象, got %T: %v", fc.Arguments, fc.Arguments)
	}
	if args["city"] != "北京" {
		t.Errorf("分片参数拼接错误: %+v", args)
	}
}

// TestClaudeToGeminiStream_MalformedToolArgs 分片拼接后仍非法时降级为空对象，
// 保留函数名而不是整段丢弃或输出非法 JSON。
func TestClaudeToGeminiStream_MalformedToolArgs(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"t1","name":"broken"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"a\":"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		``,
	}, "\n")

	events, err := runClaudeToGeminiStream(t, geminiInboundMeta(true), upstream)
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	parts := collectParts(geminiChunksOf(t, events))
	if len(parts) != 1 || parts[0].FunctionCall == nil {
		t.Fatalf("应产出 functionCall part: %+v", parts)
	}
	if parts[0].FunctionCall.FunctionName != "broken" {
		t.Errorf("函数名应保留: %+v", parts[0].FunctionCall)
	}
	if _, ok := parts[0].FunctionCall.Arguments.(map[string]any); !ok {
		t.Errorf("非法参数应降级为空对象: %v", parts[0].FunctionCall.Arguments)
	}
}

// TestClaudeToGeminiStream_UnclosedToolFlushed 上游异常断流、工具块未收到
// content_block_stop 时兜底冲刷，避免整个调用丢失。
func TestClaudeToGeminiStream_UnclosedToolFlushed(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"t1","name":"half"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"a\":1}"}}`,
		``,
		``,
	}, "\n")

	events, err := runClaudeToGeminiStream(t, geminiInboundMeta(true), upstream)
	if err != nil {
		t.Fatalf("ConvertStreamResponse error: %v", err)
	}

	parts := collectParts(geminiChunksOf(t, events))
	if len(parts) != 1 || parts[0].FunctionCall == nil || parts[0].FunctionCall.FunctionName != "half" {
		t.Fatalf("未闭合的工具块应被兜底冲刷: %+v", parts)
	}
}
