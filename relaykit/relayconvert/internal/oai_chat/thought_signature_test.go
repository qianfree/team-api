package oai_chat

import (
	"context"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

// ==================== 响应侧：OpenAI Chat → Claude 的签名写出 ====================

// 流式：消息级 thoughtSignature 附着到已打开的 thinking 块（signature_delta）。
func TestO2CStream_SignatureOnThinkingBlock(t *testing.T) {
	sse := "data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"思考\"}}]}\n\n" +
		"data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"继续\",\"thought_signature\":\"sig-think\"}}]}\n\n" +
		"data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"回答\"}}]}\n\n" +
		"data: [DONE]\n\n"
	events, err := runO2CStream(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	// 期望：signature_delta 出现在 thinking 块内（thinking_delta 之后、该块 stop 之前）。
	// 注意：事件的 Index 是指向转换器内部计数器的指针（生产路径同步序列化），
	// 事后解引用读到的是终值——此处只按事件顺序断言，不校验 Index 数值
	sigIdx, thinkStopIdx := -1, -1
	for i, e := range events {
		if e.Type == "content_block_delta" && e.Data.Delta != nil && e.Data.Delta.Type == "signature_delta" {
			sigIdx = i
			if e.Data.Delta.Signature != "sig-think" {
				t.Errorf("signature = %q, want sig-think", e.Data.Delta.Signature)
			}
		}
		if e.Type == "content_block_stop" && thinkStopIdx == -1 {
			thinkStopIdx = i
		}
	}
	if sigIdx == -1 {
		t.Fatal("未发出 signature_delta 事件")
	}
	if thinkStopIdx != -1 && sigIdx > thinkStopIdx {
		t.Errorf("signature_delta（%d）应在 thinking 块 stop（%d）之前", sigIdx, thinkStopIdx)
	}
}

// 流式：工具级签名且无 thinking 块——合成空 thinking 块承载签名，再开 tool_use 块。
func TestO2CStream_ToolSignatureSynthesizesThinkingBlock(t *testing.T) {
	sse := "data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: " + `{"id":"c","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"t1","type":"function","function":{"name":"f","arguments":"{\"a\":1}"},"thought_signature":"sig-fc"}]}}]}` + "\n\n" +
		"data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: [DONE]\n\n"
	events, err := runO2CStream(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	var types []string
	for _, e := range events {
		label := e.Type
		if e.Type == "content_block_start" && e.Data.ContentBlock != nil {
			label = "start:" + e.Data.ContentBlock.Type
		}
		if e.Type == "content_block_delta" && e.Data.Delta != nil {
			label = "delta:" + e.Data.Delta.Type
		}
		types = append(types, label)
	}
	want := []string{
		"message_start",
		"start:thinking", "delta:signature_delta", "content_block_stop", // 合成 thinking 块承载签名
		"start:tool_use", "delta:input_json_delta", "content_block_stop",
		"message_delta", "message_stop",
	}
	if len(types) != len(want) {
		t.Fatalf("event sequence = %v\nwant %v", types, want)
	}
	for i := range types {
		if types[i] != want[i] {
			t.Fatalf("event %d = %s, want %s\nfull: %v", i, types[i], want[i], types)
		}
	}
}

// 非流式：消息级签名附着到 thinking 块；无 thinking 时合成空块承载。
func TestO2CResponse_ThoughtSignature(t *testing.T) {
	rc := "思考"
	withThinking := &dto.ChatCompletionResponse{
		Model: "gemini-3-pro",
		Choices: []dto.Choice{{
			Message: dto.Message{Role: "assistant", Content: "回答", ReasoningContent: &rc, ThoughtSignature: "sig-a"},
		}},
	}
	result, _, err := (&OpenAIToClaudeResponseConverter{}).ConvertResponse(context.Background(), nil, withThinking)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	claudeResp := result.(*dto.ClaudeResponse)
	if len(claudeResp.Content) < 1 || claudeResp.Content[0].Type != "thinking" || claudeResp.Content[0].Signature != "sig-a" {
		t.Errorf("thinking 块 = %+v, want Signature=sig-a", claudeResp.Content)
	}

	// 无 thinking、工具级签名：合成空 thinking 块（thinking→text→tool_use 块序下位于首位）
	fcOnly := &dto.ChatCompletionResponse{
		Model: "gemini-3-pro",
		Choices: []dto.Choice{{
			FinishReason: "tool_calls",
			Message: dto.Message{
				Role:    "assistant",
				Content: "",
				ToolCalls: []dto.ToolCall{
					{ID: "t1", Type: "function", Function: dto.FunctionCall{Name: "f", Arguments: "{}"}, ThoughtSignature: "sig-fc"},
				},
			},
		}},
	}
	result, _, err = (&OpenAIToClaudeResponseConverter{}).ConvertResponse(context.Background(), nil, fcOnly)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	claudeResp = result.(*dto.ClaudeResponse)
	if len(claudeResp.Content) < 2 || claudeResp.Content[0].Type != "thinking" ||
		claudeResp.Content[0].Signature != "sig-fc" || claudeResp.Content[1].Type != "tool_use" {
		t.Errorf("合成 thinking 块缺失或块序错误：%+v", claudeResp.Content)
	}
}

// ==================== 请求侧：Claude → OpenAI Chat 的签名捕获 ====================

// assistant thinking 块的 signature 捕获为消息级 ThoughtSignature（含空 thinking 承载块）。
func TestC2ORequest_ThoughtSignatureCapture(t *testing.T) {
	req := &dto.ClaudeRequest{
		Model: "gemini-3-pro",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "查天气"},
			{Role: "assistant", Content: []any{
				map[string]any{"type": "thinking", "thinking": "思考", "signature": "sig-a"},
				map[string]any{"type": "tool_use", "id": "t1", "name": "get_weather", "input": map[string]any{"city": "北京"}},
			}},
			{Role: "user", Content: []any{
				map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": "晴"},
			}},
			// 空 thinking 承载块（未开 includeThoughts 的 Gemini 3 函数调用形态）
			{Role: "assistant", Content: []any{
				map[string]any{"type": "thinking", "thinking": "", "signature": "sig-b"},
				map[string]any{"type": "text", "text": "北京是晴天"},
			}},
		},
	}

	result, err := (&ClaudeToOpenAIRequestConverter{}).ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	openaiReq := result.(*dto.GeneralOpenAIRequest)

	var assistants []dto.Message
	for _, m := range openaiReq.Messages {
		if m.Role == "assistant" {
			assistants = append(assistants, m)
		}
	}
	if len(assistants) != 2 {
		t.Fatalf("assistant 消息数 = %d, want 2", len(assistants))
	}
	if assistants[0].ThoughtSignature != "sig-a" {
		t.Errorf("首条 assistant ThoughtSignature = %q, want sig-a", assistants[0].ThoughtSignature)
	}
	if assistants[1].ThoughtSignature != "sig-b" {
		t.Errorf("空 thinking 承载块的签名未捕获：ThoughtSignature = %q, want sig-b", assistants[1].ThoughtSignature)
	}
}
