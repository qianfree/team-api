package oai_chat

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

func TestClaudeToOpenAIStreamConverter_Metadata(t *testing.T) {
	converter := &ClaudeToOpenAIStreamConverter{}

	if converter.ID() != relayconvert.ConverterClaudeMessagesToOpenAIChatStream {
		t.Errorf("ID() = %q, want %q", converter.ID(), relayconvert.ConverterClaudeMessagesToOpenAIChatStream)
	}

	if converter.From() != types.RelayFormatClaude {
		t.Errorf("From() = %q, want %q", converter.From(), types.RelayFormatClaude)
	}

	if converter.To() != types.RelayFormatOpenAI {
		t.Errorf("To() = %q, want %q", converter.To(), types.RelayFormatOpenAI)
	}

	if converter.Quality() != relayconvert.ResponseConverterQualityGood {
		t.Errorf("Quality() = %q, want %q", converter.Quality(), relayconvert.ResponseConverterQualityGood)
	}
}

func TestClaudeToOpenAIStreamConverter_BasicStream(t *testing.T) {
	converter := &ClaudeToOpenAIStreamConverter{}
	ctx := context.Background()

	// 模拟 Claude SSE 流
	claudeStream := `data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-3-opus-20240229","usage":{"input_tokens":10,"output_tokens":0}}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":", how can I help you?"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}

data: {"type":"message_stop"}

`

	reader := strings.NewReader(claudeStream)
	info := &mockMeta{
		originModel: "gpt-4",
	}

	var chunks []*dto.ChatCompletionStreamResponse
	chunkWriter := func(chunk any) error {
		if c, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			chunks = append(chunks, c)
		}
		return nil
	}

	err := converter.ConvertStreamResponse(ctx, info, reader, chunkWriter)
	if err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	// 校验 chunks
	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}

	// 首个 chunk 应包含 role
	if chunks[0].Choices[0].Delta.Role != "assistant" {
		t.Errorf("First chunk role = %q, want %q", chunks[0].Choices[0].Delta.Role, "assistant")
	}

	// 收集 text chunks
	var textContent string
	for _, chunk := range chunks {
		if chunk.Choices[0].Delta.Content != nil {
			if s, ok := chunk.Choices[0].Delta.Content.(string); ok {
				textContent += s
			}
		}
	}

	expectedText := "Hello, how can I help you?"
	if textContent != expectedText {
		t.Errorf("Text content = %q, want %q", textContent, expectedText)
	}

	// 末尾 chunk 应包含 finish_reason 和 usage
	lastChunk := chunks[len(chunks)-1]
	if lastChunk.Choices[0].FinishReason == nil || *lastChunk.Choices[0].FinishReason != "stop" {
		t.Errorf("Last chunk finish_reason = %v, want %q", lastChunk.Choices[0].FinishReason, "stop")
	}

	if lastChunk.Usage == nil {
		t.Fatal("Last chunk should have usage")
	}

	if lastChunk.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", lastChunk.Usage.PromptTokens)
	}

	if lastChunk.Usage.CompletionTokens != 7 {
		t.Errorf("CompletionTokens = %d, want 7", lastChunk.Usage.CompletionTokens)
	}
}

func TestClaudeToOpenAIStreamConverter_ThinkingStream(t *testing.T) {
	converter := &ClaudeToOpenAIStreamConverter{}
	ctx := context.Background()

	claudeStream := `data: {"type":"message_start","message":{"id":"msg_456","type":"message","role":"assistant","model":"claude-3-opus-20240229","usage":{"input_tokens":15,"output_tokens":0}}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me think..."}}

data: {"type":"content_block_stop","index":0}

data: {"type":"content_block_start","index":1,"content_block":{"type":"text"}}

data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"The answer is 42"}}

data: {"type":"content_block_stop","index":1}

data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":20}}

data: {"type":"message_stop"}

`

	reader := strings.NewReader(claudeStream)

	var chunks []*dto.ChatCompletionStreamResponse
	chunkWriter := func(chunk any) error {
		if c, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			chunks = append(chunks, c)
		}
		return nil
	}

	err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter)
	if err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	// 收集 thinking 内容
	var thinkingContent string
	for _, chunk := range chunks {
		if chunk.Choices[0].Delta.ReasoningContent != nil {
			thinkingContent += *chunk.Choices[0].Delta.ReasoningContent
		}
	}

	if thinkingContent != "Let me think..." {
		t.Errorf("Thinking content = %q, want %q", thinkingContent, "Let me think...")
	}

	// 收集 text 内容
	var textContent string
	for _, chunk := range chunks {
		if chunk.Choices[0].Delta.Content != nil {
			if s, ok := chunk.Choices[0].Delta.Content.(string); ok {
				textContent += s
			}
		}
	}

	if textContent != "The answer is 42" {
		t.Errorf("Text content = %q, want %q", textContent, "The answer is 42")
	}
}

func TestClaudeToOpenAIStreamConverter_ToolCalls(t *testing.T) {
	converter := &ClaudeToOpenAIStreamConverter{}
	ctx := context.Background()

	claudeStream := `data: {"type":"message_start","message":{"id":"msg_789","type":"message","role":"assistant","model":"claude-3-opus-20240229","usage":{"input_tokens":20,"output_tokens":0}}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_abc","name":"get_weather"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"San Francisco\"}"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":10}}

data: {"type":"message_stop"}

`

	reader := strings.NewReader(claudeStream)

	var chunks []*dto.ChatCompletionStreamResponse
	chunkWriter := func(chunk any) error {
		if c, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			chunks = append(chunks, c)
		}
		return nil
	}

	err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter)
	if err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	// 收集 tool call chunks
	var toolCallID string
	var toolCallName string
	var toolCallArgs string

	for _, chunk := range chunks {
		if len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			tc := chunk.Choices[0].Delta.ToolCalls[0]
			if tc.ID != "" {
				toolCallID = tc.ID
			}
			if tc.Function.Name != "" {
				toolCallName = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				toolCallArgs += tc.Function.Arguments
			}
		}
	}

	if toolCallID != "call_abc" {
		t.Errorf("Tool call ID = %q, want %q", toolCallID, "call_abc")
	}

	if toolCallName != "get_weather" {
		t.Errorf("Tool call name = %q, want %q", toolCallName, "get_weather")
	}

	expectedArgs := `{"city":"San Francisco"}`
	if toolCallArgs != expectedArgs {
		t.Errorf("Tool call arguments = %q, want %q", toolCallArgs, expectedArgs)
	}

	// 末尾 chunk 应有 finish_reason = "tool_calls"
	lastChunk := chunks[len(chunks)-1]
	if lastChunk.Choices[0].FinishReason == nil || *lastChunk.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("Finish reason = %v, want %q", lastChunk.Choices[0].FinishReason, "tool_calls")
	}
}

func TestClaudeToOpenAIStreamConverter_ErrorEvent(t *testing.T) {
	converter := &ClaudeToOpenAIStreamConverter{}
	ctx := context.Background()

	claudeStream := `data: {"type":"message_start","message":{"id":"msg_err","type":"message","role":"assistant","model":"claude-3-opus-20240229","usage":{"input_tokens":5,"output_tokens":0}}}

data: {"type":"error","error":{"type":"overloaded_error","message":"Server overloaded"}}

`

	reader := strings.NewReader(claudeStream)

	var chunks []*dto.ChatCompletionStreamResponse
	chunkWriter := func(chunk any) error {
		if c, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			chunks = append(chunks, c)
		}
		return nil
	}

	err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter)
	if err == nil {
		t.Fatal("Expected error for error event, got nil")
	}

	if !strings.Contains(err.Error(), "claude stream error") {
		t.Errorf("Error message = %q, should contain %q", err.Error(), "claude stream error")
	}
}

func TestClaudeToOpenAIStreamConverter_ContextCancellation(t *testing.T) {
	converter := &ClaudeToOpenAIStreamConverter{}
	ctx, cancel := context.WithCancel(context.Background())

	claudeStream := `data: {"type":"message_start","message":{"id":"msg_cancel","type":"message","role":"assistant","model":"claude-3-opus-20240229","usage":{"input_tokens":5,"output_tokens":0}}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}

`

	reader := strings.NewReader(claudeStream)

	chunkCount := 0
	chunkWriter := func(chunk any) error {
		chunkCount++
		if chunkCount == 1 {
			cancel() // 在首个 chunk 后取消
		}
		return nil
	}

	err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter)
	if err == nil {
		t.Fatal("Expected context.Canceled error, got nil")
	}

	if err != context.Canceled {
		t.Errorf("Error = %v, want context.Canceled", err)
	}
}

func TestClaudeToOpenAIStreamConverter_ChunkStructure(t *testing.T) {
	converter := &ClaudeToOpenAIStreamConverter{}
	ctx := context.Background()

	claudeStream := `data: {"type":"message_start","message":{"id":"msg_struct","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":100,"output_tokens":0}}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Test"}}

data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}

data: {"type":"message_stop"}

`

	reader := strings.NewReader(claudeStream)
	info := &mockMeta{
		originModel: "gpt-4-turbo",
	}

	var chunks []*dto.ChatCompletionStreamResponse
	chunkWriter := func(chunk any) error {
		if c, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			chunks = append(chunks, c)
		}
		return nil
	}

	err := converter.ConvertStreamResponse(ctx, info, reader, chunkWriter)
	if err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	// 校验所有 chunk 的结构是否正确
	for i, chunk := range chunks {
		if chunk.ID == "" {
			t.Errorf("Chunk %d: ID is empty", i)
		}

		if chunk.Object != "chat.completion.chunk" {
			t.Errorf("Chunk %d: Object = %q, want %q", i, chunk.Object, "chat.completion.chunk")
		}

		if chunk.Created == 0 {
			t.Errorf("Chunk %d: Created is 0", i)
		}

		if chunk.Model != "claude-3-5-sonnet-20241022" {
			t.Errorf("Chunk %d: Model = %q, want %q", i, chunk.Model, "claude-3-5-sonnet-20241022")
		}

		if len(chunk.Choices) != 1 {
			t.Errorf("Chunk %d: Expected 1 choice, got %d", i, len(chunk.Choices))
		}

		if chunk.Choices[0].Index != 0 {
			t.Errorf("Chunk %d: Choice index = %d, want 0", i, chunk.Choices[0].Index)
		}
	}

	// 末尾 chunk 应包含 usage
	lastChunk := chunks[len(chunks)-1]
	if lastChunk.Usage == nil {
		t.Error("Last chunk should have usage")
	} else {
		if lastChunk.Usage.PromptTokens != 100 {
			t.Errorf("PromptTokens = %d, want 100", lastChunk.Usage.PromptTokens)
		}
		if lastChunk.Usage.CompletionTokens != 1 {
			t.Errorf("CompletionTokens = %d, want 1", lastChunk.Usage.CompletionTokens)
		}
		if lastChunk.Usage.TotalTokens != 101 {
			t.Errorf("TotalTokens = %d, want 101", lastChunk.Usage.TotalTokens)
		}
	}
}

func TestClaudeToOpenAIStreamConverter_EmptyStream(t *testing.T) {
	converter := &ClaudeToOpenAIStreamConverter{}
	ctx := context.Background()

	claudeStream := ``

	reader := strings.NewReader(claudeStream)

	var chunks []*dto.ChatCompletionStreamResponse
	chunkWriter := func(chunk any) error {
		if c, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			chunks = append(chunks, c)
		}
		return nil
	}

	err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter)
	if err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty stream, got %d", len(chunks))
	}
}

func TestClaudeToOpenAIStreamConverter_MalformedJSON(t *testing.T) {
	converter := &ClaudeToOpenAIStreamConverter{}
	ctx := context.Background()

	claudeStream := `data: {"type":"message_start","message":{"id":"msg_test"

data: invalid json here

data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}

data: {"type":"message_stop"}

`

	reader := strings.NewReader(claudeStream)

	var chunks []*dto.ChatCompletionStreamResponse
	chunkWriter := func(chunk any) error {
		if c, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			chunks = append(chunks, c)
		}
		return nil
	}

	// 对格式错误的 JSON 不应报错，仅跳过这些行
	err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter)
	if err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	// 仍应处理有效的 chunk
	if len(chunks) == 0 {
		t.Error("Expected some chunks despite malformed JSON lines")
	}
}

// 辅助函数：构造一个接近真实的 SSE 字节流
func createClaudeSSEStream(events []map[string]any) *bytes.Buffer {
	var buf bytes.Buffer
	for _, event := range events {
		data, _ := json.Marshal(event)
		buf.WriteString("data: ")
		buf.Write(data)
		buf.WriteString("\n\n")
	}
	return &buf
}
