package register

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// 思考签名往返闭环回归（codex CLI × Claude / Gemini 上游）。
//
// 背景：Anthropic 强制要求「thinking 开启时，带 tool_use 的 assistant 消息必须以
// thinking 块开头」；Gemini 3 强制要求函数调用轮回传 thoughtSignature。Responses 协议
// 侧唯一可用的载体是 reasoning 项的 encrypted_content——codex 会原样回传该项。
// 修复前响应侧丢签名、请求侧无还原、o2c 不重建 thinking 块，导致 codex 打 Claude 渠道
// 从第二轮（首次工具调用之后）起必 400。

func sigRoundtripMeta() *convmeta.Values {
	return &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "claude-sonnet-4-5",
		UpstreamModelName:   "claude-sonnet-4-5-20250929",
		IsStream:            true,
		Options: &convmeta.Options{
			Claude: convmeta.ClaudeOptions{DefaultMaxTokens: func(string) int { return 4096 }},
		},
	}
}

// collectResponsesEvents 跑一条流式转换器并收集 Responses 事件。
func collectResponsesEvents(t *testing.T, from types.RelayFormat, sse string) []*dto.ResponsesStreamEvent {
	t.Helper()
	fn, _, ok := relayconvert.LookupStreamConverter(from, types.RelayFormatOpenAIResponses)
	if !ok {
		t.Fatalf("stream converter %s->responses not registered", from)
	}
	var events []*dto.ResponsesStreamEvent
	err := fn(context.Background(), sigRoundtripMeta(), strings.NewReader(sse), func(chunk any) error {
		if ev, ok := chunk.(*dto.ResponsesStreamEvent); ok {
			events = append(events, ev)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream convert: %v", err)
	}
	return events
}

// reasoningItemEncryptedContent 取首个 reasoning output_item.done 的 encrypted_content。
func reasoningItemEncryptedContent(events []*dto.ResponsesStreamEvent) string {
	for _, ev := range events {
		if ev.Type != "response.output_item.done" {
			continue
		}
		data, ok := ev.Data.(map[string]any)
		if !ok {
			continue
		}
		item, ok := data["item"].(map[string]any)
		if !ok || item["type"] != "reasoning" {
			continue
		}
		if enc, ok := item["encrypted_content"].(string); ok {
			return enc
		}
	}
	return ""
}

// codexTurn2Input 构造 codex 第二轮（工具调用后）的 input 数组。
// signature 非空时带上 reasoning 项的 encrypted_content（模拟 codex 原样回传）。
func codexTurn2Input(t *testing.T, thinkingText, signature string) json.RawMessage {
	t.Helper()
	input := []map[string]any{
		{"type": "message", "role": "user", "content": []map[string]any{
			{"type": "input_text", "text": "list files"},
		}},
	}
	if signature != "" {
		input = append(input, map[string]any{
			"type": "reasoning", "id": "rs_1", "encrypted_content": signature,
			"summary": []map[string]any{{"type": "summary_text", "text": thinkingText}},
		})
	}
	input = append(input,
		map[string]any{"type": "function_call", "call_id": "call_1", "name": "shell",
			"arguments": `{"command":["ls"]}`},
		map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "a.txt"},
	)
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	return raw
}

// TestSignatureRoundtrip_ClaudeToCodexAndBack Claude thinking 签名 → Responses
// encrypted_content → codex 回传 → 重建 Claude thinking 块的完整闭环。
func TestSignatureRoundtrip_ClaudeToCodexAndBack(t *testing.T) {
	const (
		wantSig      = "ErUBCkYIBRgCKkD1"
		wantThinking = "The user wants a file listing. I should run ls."
	)

	// ① Claude 上游流：thinking(+signature) → tool_use
	claudeSSE := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_x","model":"claude-sonnet-4-5","usage":{"input_tokens":10,"output_tokens":0}}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking"}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"` + wantThinking + `"}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"` + wantSig + `"}}`,
		`data: {"type":"content_block_stop","index":0}`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"shell"}}`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"command\":[\"ls\"]}"}}`,
		`data: {"type":"content_block_stop","index":1}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":25}}`,
		`data: {"type":"message_stop"}`,
	}, "\n\n") + "\n\n"

	events := collectResponsesEvents(t, types.RelayFormatClaude, claudeSSE)
	if gotSig := reasoningItemEncryptedContent(events); gotSig != wantSig {
		t.Fatalf("reasoning 项未透传 thinking 签名: encrypted_content = %q, want %q", gotSig, wantSig)
	}

	// ② codex 第二轮请求：原样回传 reasoning 项（含 encrypted_content）+ 工具结果
	req := &dto.OpenAIResponsesRequest{
		Model:     "claude-sonnet-4-5",
		Input:     codexTurn2Input(t, wantThinking, wantSig),
		Reasoning: &dto.Reasoning{Effort: "medium"},
	}

	spec, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterOpenAIResponsesToClaudeMessages)
	if !ok {
		t.Fatal("responses->claude chain not registered")
	}
	out, err := relayconvert.ExecuteRequestConverter(context.Background(), spec, sigRoundtripMeta(), req)
	if err != nil {
		t.Fatalf("responses->claude: %v", err)
	}
	claudeReq, ok := out.(*dto.ClaudeRequest)
	if !ok {
		t.Fatalf("result type = %T, want *dto.ClaudeRequest", out)
	}

	// ③ thinking 仍开启，且带 tool_use 的 assistant 消息以 thinking 块开头（含原签名）
	if claudeReq.Thinking == nil || claudeReq.Thinking.Type != "enabled" {
		t.Fatalf("thinking = %+v, want enabled（签名齐备时不应回退关闭）", claudeReq.Thinking)
	}
	blocks := toolUseMessageBlocks(t, claudeReq)
	if blocks[0].Type != "thinking" {
		t.Fatalf("带 tool_use 的 assistant 消息首块 = %q, want thinking（否则 Anthropic 400）", blocks[0].Type)
	}
	if blocks[0].Signature != wantSig {
		t.Errorf("thinking.signature = %q, want %q", blocks[0].Signature, wantSig)
	}
	if blocks[0].Thinking == nil || *blocks[0].Thinking != wantThinking {
		t.Errorf("thinking 文本必须与签名产出时逐字一致（否则上游验签失败），got %v", blocks[0].Thinking)
	}
}

// toolUseMessageBlocks 返回最后一条含 tool_use 块的消息的内容块。
func toolUseMessageBlocks(t *testing.T, req *dto.ClaudeRequest) []dto.ClaudeContentBlock {
	t.Helper()
	var found []dto.ClaudeContentBlock
	for i := range req.Messages {
		blocks, ok := req.Messages[i].Content.([]dto.ClaudeContentBlock)
		if !ok {
			continue
		}
		for _, b := range blocks {
			if b.Type == "tool_use" {
				found = blocks
			}
		}
	}
	if found == nil {
		t.Fatal("未找到带 tool_use 的 assistant 消息")
	}
	return found
}

// TestSignatureMissing_FallsBackToThinkingDisabled 签名不可得时（客户端未回传
// encrypted_content）不得留下「thinking 开启 + 无 thinking 块的 tool_use 轮」这一必 400
// 组合，应回退关闭 thinking 让 agent 循环继续。
func TestSignatureMissing_FallsBackToThinkingDisabled(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:     "claude-sonnet-4-5",
		Input:     codexTurn2Input(t, "", ""),
		Reasoning: &dto.Reasoning{Effort: "medium"},
	}

	spec, _ := relayconvert.LookupRequestConverter(relayconvert.ConverterOpenAIResponsesToClaudeMessages)
	out, err := relayconvert.ExecuteRequestConverter(context.Background(), spec, sigRoundtripMeta(), req)
	if err != nil {
		t.Fatalf("responses->claude: %v", err)
	}
	claudeReq := out.(*dto.ClaudeRequest)
	if claudeReq.Thinking != nil {
		t.Errorf("thinking = %+v, want nil（无签名的历史工具轮 + thinking 开启 = 上游必 400）",
			claudeReq.Thinking)
	}
}

// TestSignatureRoundtrip_GeminiThoughtSignatureToCodex Gemini thoughtSignature 经
// gemini→openai→responses 链透传到 reasoning 项的 encrypted_content，并能在下一轮
// 还原为 chat 中间格式的 thought_signature（o2g 据此回挂到 functionCall part）。
func TestSignatureRoundtrip_GeminiThoughtSignatureToCodex(t *testing.T) {
	const wantSig = "CvkBAdHtim"
	geminiSSE := `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"I should run ls.","thought":true,"thoughtSignature":"` + wantSig + `"}]},"index":0}]}` + "\n\n" +
		`data: {"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"shell","args":{"command":["ls"]}}}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":25,"totalTokenCount":35}}` + "\n\n"

	events := collectResponsesEvents(t, types.RelayFormatGemini, geminiSSE)
	if got := reasoningItemEncryptedContent(events); got != wantSig {
		t.Fatalf("reasoning 项未透传 thoughtSignature: encrypted_content = %q, want %q", got, wantSig)
	}

	// 回传：reasoning.encrypted_content → chat thought_signature
	req := &dto.OpenAIResponsesRequest{
		Model: "gemini-3-pro-preview",
		Input: codexTurn2Input(t, "I should run ls.", wantSig),
	}
	spec, _ := relayconvert.LookupRequestConverter(relayconvert.ConverterOpenAIResponsesToOpenAIChat)
	out, err := relayconvert.ExecuteRequestConverter(context.Background(), spec,
		&convmeta.Values{ChannelMetaAttached: true, UpstreamModelName: "gemini-3-pro-preview"}, req)
	if err != nil {
		t.Fatalf("responses->chat: %v", err)
	}
	chat := out.(*dto.GeneralOpenAIRequest)
	found := false
	for _, m := range chat.Messages {
		if m.Role == "assistant" && m.ThoughtSignature == wantSig {
			found = true
		}
	}
	if !found {
		t.Errorf("assistant 消息未还原 thought_signature=%q（Gemini 3 函数调用轮缺签名会被上游 400）", wantSig)
	}
}
