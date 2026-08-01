package oai_gemini

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

func TestGeminiToOpenAIResponseConverter_Metadata(t *testing.T) {
	converter := &GeminiToOpenAIResponseConverter{}

	if converter.ID() != relayconvert.ResponseConverterGeminiChatToOAIChat {
		t.Errorf("ID() = %q, want %q", converter.ID(), relayconvert.ResponseConverterGeminiChatToOAIChat)
	}

	if converter.From() != types.RelayFormatGemini {
		t.Errorf("From() = %q, want %q", converter.From(), types.RelayFormatGemini)
	}

	if converter.To() != types.RelayFormatOpenAI {
		t.Errorf("To() = %q, want %q", converter.To(), types.RelayFormatOpenAI)
	}

	if converter.Quality() != relayconvert.ResponseConverterQualityGood {
		t.Errorf("Quality() = %q, want %q", converter.Quality(), relayconvert.ResponseConverterQualityGood)
	}
}

func TestGeminiToOpenAIResponseConverter_BasicConversion(t *testing.T) {
	converter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

	geminiResp := &dto.GeminiChatResponse{
		ModelName: "gemini-pro",
		Candidates: []dto.GeminiCandidate{
			{
				Index:        0,
				FinishReason: "STOP",
				Content: &dto.GeminiContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "Hello! How can I help you today?"},
					},
				},
			},
		},
		UsageMetadata: &dto.GeminiUsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 20,
			TotalTokenCount:      30,
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, geminiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp, ok := result.(*dto.ChatCompletionResponse)
	if !ok {
		t.Fatalf("Expected *dto.ChatCompletionResponse, got %T", result)
	}

	// Check basic metadata
	if openaiResp.Model != "gemini-pro" {
		t.Errorf("Model = %q, want %q", openaiResp.Model, "gemini-pro")
	}
	if openaiResp.Object != "chat.completion" {
		t.Errorf("Object = %q, want %q", openaiResp.Object, "chat.completion")
	}
	if len(openaiResp.Choices) != 1 {
		t.Fatalf("Choices count = %d, want 1", len(openaiResp.Choices))
	}

	// Check choice
	choice := openaiResp.Choices[0]
	if choice.Index != 0 {
		t.Errorf("Choice index = %d, want 0", choice.Index)
	}
	if choice.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want %q", choice.FinishReason, "stop")
	}

	// Check message
	if choice.Message.Role != "assistant" {
		t.Errorf("Message role = %q, want %q", choice.Message.Role, "assistant")
	}
	content, ok := choice.Message.Content.(string)
	if !ok {
		t.Fatalf("Message content is not string, got %T", choice.Message.Content)
	}
	if content != "Hello! How can I help you today?" {
		t.Errorf("Message content = %q, want %q", content, "Hello! How can I help you today?")
	}

	// Check usage
	if openaiResp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", openaiResp.Usage.PromptTokens)
	}
	if openaiResp.Usage.CompletionTokens != 20 {
		t.Errorf("CompletionTokens = %d, want 20", openaiResp.Usage.CompletionTokens)
	}
	if openaiResp.Usage.TotalTokens != 30 {
		t.Errorf("TotalTokens = %d, want 30", openaiResp.Usage.TotalTokens)
	}
}

func TestGeminiToOpenAIResponseConverter_SafetyBlock(t *testing.T) {
	converter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

	geminiResp := &dto.GeminiChatResponse{
		PromptFeedback: &dto.GeminiPromptFeedback{
			BlockReason: "SAFETY",
		},
	}

	_, err := converter.ConvertResponse(ctx, nil, geminiResp)
	if err == nil {
		t.Fatal("Expected error for safety block, got nil")
	}
	if !strings.Contains(err.Error(), "SAFETY") {
		t.Errorf("Error message should contain SAFETY, got %q", err.Error())
	}
}

func TestGeminiToOpenAIResponseConverter_MultimodalContent(t *testing.T) {
	converter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

	geminiResp := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{
			{
				Index:        0,
				FinishReason: "STOP",
				Content: &dto.GeminiContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "This is an image:"},
						{
							InlineData: &dto.GeminiInlineData{
								MimeType: "image/png",
								Data:     "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
							},
						},
						{Text: "And here's a file:"},
						{
							FileData: &dto.GeminiFileData{
								FileURI: "gs://bucket/file.pdf",
							},
						},
					},
				},
			},
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, geminiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp, ok := result.(*dto.ChatCompletionResponse)
	if !ok {
		t.Fatalf("Expected *dto.ChatCompletionResponse, got %T", result)
	}

	content, ok := openaiResp.Choices[0].Message.Content.(string)
	if !ok {
		t.Fatalf("Message content is not string, got %T", openaiResp.Choices[0].Message.Content)
	}

	// Should contain image markdown and file link
	if !strings.Contains(content, "![image](data:image/png;base64,") {
		t.Error("Content should contain image markdown")
	}
	if !strings.Contains(content, "[file](gs://bucket/file.pdf)") {
		t.Error("Content should contain file link")
	}
}

func TestGeminiToOpenAIResponseConverter_ExecutableCode(t *testing.T) {
	converter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

	geminiResp := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{
			{
				Index:        0,
				FinishReason: "STOP",
				Content: &dto.GeminiContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{
							ExecutableCode: &dto.GeminiExecutableCode{
								Language: "python",
								Code:     "print('Hello, World!')",
							},
						},
						{
							CodeExecutionResult: &dto.GeminiCodeExecutionResult{
								Outcome: "OUTCOME_OK",
								Output:  "Hello, World!\n",
							},
						},
					},
				},
			},
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, geminiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp, ok := result.(*dto.ChatCompletionResponse)
	if !ok {
		t.Fatalf("Expected *dto.ChatCompletionResponse, got %T", result)
	}

	content, ok := openaiResp.Choices[0].Message.Content.(string)
	if !ok {
		t.Fatalf("Message content is not string, got %T", openaiResp.Choices[0].Message.Content)
	}

	// Should contain code block and execution result
	if !strings.Contains(content, "```python") {
		t.Error("Content should contain code block with language")
	}
	if !strings.Contains(content, "print('Hello, World!')") {
		t.Error("Content should contain the code")
	}
	if !strings.Contains(content, "Execution OUTCOME_OK") {
		t.Error("Content should contain execution result")
	}
}

func TestGeminiToOpenAIResponseConverter_ToolCalls(t *testing.T) {
	converter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

	geminiResp := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{
			{
				Index:        0,
				FinishReason: "STOP",
				Content: &dto.GeminiContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{
							FunctionCall: &dto.GeminiFunctionCall{
								FunctionName: "get_weather",
								Arguments: map[string]any{
									"location": "San Francisco",
									"unit":     "celsius",
								},
							},
						},
					},
				},
			},
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, geminiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp, ok := result.(*dto.ChatCompletionResponse)
	if !ok {
		t.Fatalf("Expected *dto.ChatCompletionResponse, got %T", result)
	}

	choice := openaiResp.Choices[0]
	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("ToolCalls count = %d, want 1", len(choice.Message.ToolCalls))
	}

	toolCall := choice.Message.ToolCalls[0]
	if toolCall.Type != "function" {
		t.Errorf("ToolCall type = %q, want %q", toolCall.Type, "function")
	}
	if toolCall.Function.Name != "get_weather" {
		t.Errorf("Function name = %q, want %q", toolCall.Function.Name, "get_weather")
	}

	// Parse arguments
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		t.Fatalf("Failed to parse arguments: %v", err)
	}
	if args["location"] != "San Francisco" {
		t.Errorf("Location = %v, want %q", args["location"], "San Francisco")
	}
	if args["unit"] != "celsius" {
		t.Errorf("Unit = %v, want %q", args["unit"], "celsius")
	}

	// Finish reason should be "tool_calls"
	if choice.FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want %q", choice.FinishReason, "tool_calls")
	}
}

func TestGeminiToOpenAIResponseConverter_ThinkingContent(t *testing.T) {
	converter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

	thoughtTrue := true
	geminiResp := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{
			{
				Index:        0,
				FinishReason: "STOP",
				Content: &dto.GeminiContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "Let me think about this."},
						{Thought: &thoughtTrue, Text: "This requires careful analysis."},
						{Text: "The answer is 42."},
					},
				},
			},
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, geminiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp, ok := result.(*dto.ChatCompletionResponse)
	if !ok {
		t.Fatalf("Expected *dto.ChatCompletionResponse, got %T", result)
	}

	message := openaiResp.Choices[0].Message

	// Regular content should contain non-thought text
	content, ok := message.Content.(string)
	if !ok {
		t.Fatalf("Message content is not string, got %T", message.Content)
	}
	if !strings.Contains(content, "Let me think about this.") {
		t.Error("Content should contain first text")
	}
	if !strings.Contains(content, "The answer is 42.") {
		t.Error("Content should contain third text")
	}

	// ReasoningContent should contain thought text
	if message.ReasoningContent == nil {
		t.Fatal("ReasoningContent is nil")
	}
	if !strings.Contains(*message.ReasoningContent, "This requires careful analysis.") {
		t.Errorf("ReasoningContent = %q, should contain thought text", *message.ReasoningContent)
	}
}

func TestGeminiToOpenAIResponseConverter_FinishReasonMapping(t *testing.T) {
	tests := []struct {
		name         string
		geminiReason string
		openaiReason string
	}{
		{"STOP to stop", "STOP", "stop"},
		{"MAX_TOKENS to length", "MAX_TOKENS", "length"},
		{"SAFETY to content_filter", "SAFETY", "content_filter"},
		{"RECITATION to content_filter", "RECITATION", "content_filter"},
		{"BLOCKLIST to content_filter", "BLOCKLIST", "content_filter"},
		{"PROHIBITED_CONTENT to content_filter", "PROHIBITED_CONTENT", "content_filter"},
		{"SPII to content_filter", "SPII", "content_filter"},
		{"OTHER to stop", "OTHER", "stop"},
		{"empty to stop", "", "stop"},
	}

	converter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			geminiResp := &dto.GeminiChatResponse{
				Candidates: []dto.GeminiCandidate{
					{
						Index:        0,
						FinishReason: tt.geminiReason,
						Content:      &dto.GeminiContent{Role: "model"},
					},
				},
			}

			result, err := converter.ConvertResponse(ctx, nil, geminiResp)
			if err != nil {
				t.Fatalf("ConvertResponse failed: %v", err)
			}

			openaiResp, ok := result.(*dto.ChatCompletionResponse)
			if !ok {
				t.Fatalf("Expected *dto.ChatCompletionResponse, got %T", result)
			}

			if openaiResp.Choices[0].FinishReason != tt.openaiReason {
				t.Errorf("FinishReason = %q, want %q", openaiResp.Choices[0].FinishReason, tt.openaiReason)
			}
		})
	}
}

func TestGeminiToOpenAIResponseConverter_InvalidResponseType(t *testing.T) {
	converter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

	_, err := converter.ConvertResponse(ctx, nil, "not a response")
	if err == nil {
		t.Fatal("Expected error for invalid response type, got nil")
	}
}

func TestGeminiToOpenAIResponseConverter_EmptyCandidates(t *testing.T) {
	converter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

	geminiResp := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{},
	}

	result, err := converter.ConvertResponse(ctx, nil, geminiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp, ok := result.(*dto.ChatCompletionResponse)
	if !ok {
		t.Fatalf("Expected *dto.ChatCompletionResponse, got %T", result)
	}

	if len(openaiResp.Choices) != 0 {
		t.Errorf("Choices count = %d, want 0", len(openaiResp.Choices))
	}
}

func TestGeminiToOpenAIResponseConverter_MultipleToolCalls(t *testing.T) {
	converter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

	geminiResp := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{
			{
				Index:        0,
				FinishReason: "STOP",
				Content: &dto.GeminiContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{
							FunctionCall: &dto.GeminiFunctionCall{
								FunctionName: "get_weather",
								Arguments:    map[string]any{"location": "NYC"},
							},
						},
						{
							FunctionCall: &dto.GeminiFunctionCall{
								FunctionName: "get_time",
								Arguments:    map[string]any{"timezone": "EST"},
							},
						},
					},
				},
			},
		},
	}

	result, err := converter.ConvertResponse(ctx, nil, geminiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	openaiResp, ok := result.(*dto.ChatCompletionResponse)
	if !ok {
		t.Fatalf("Expected *dto.ChatCompletionResponse, got %T", result)
	}

	toolCalls := openaiResp.Choices[0].Message.ToolCalls
	if len(toolCalls) != 2 {
		t.Fatalf("ToolCalls count = %d, want 2", len(toolCalls))
	}

	// Check first tool call
	if toolCalls[0].Function.Name != "get_weather" {
		t.Errorf("First function name = %q, want %q", toolCalls[0].Function.Name, "get_weather")
	}

	// Check second tool call
	if toolCalls[1].Function.Name != "get_time" {
		t.Errorf("Second function name = %q, want %q", toolCalls[1].Function.Name, "get_time")
	}

	// Tool call IDs should be different
	if toolCalls[0].ID == toolCalls[1].ID {
		t.Error("Tool call IDs should be different")
	}
}

func TestGeminiToOpenAIStreamConverter_Metadata(t *testing.T) {
	converter := &GeminiToOpenAIStreamConverter{}

	if converter.ID() != relayconvert.ResponseConverterGeminiChatToOAIChatStream {
		t.Errorf("ID() = %q, want %q", converter.ID(), relayconvert.ResponseConverterGeminiChatToOAIChatStream)
	}

	if converter.From() != types.RelayFormatGemini {
		t.Errorf("From() = %q, want %q", converter.From(), types.RelayFormatGemini)
	}

	if converter.To() != types.RelayFormatOpenAI {
		t.Errorf("To() = %q, want %q", converter.To(), types.RelayFormatOpenAI)
	}

	if converter.Quality() != relayconvert.ResponseConverterQualityGood {
		t.Errorf("Quality() = %q, want %q", converter.Quality(), relayconvert.ResponseConverterQualityGood)
	}
}

func TestGeminiToOpenAIStreamConverter_BasicStreaming(t *testing.T) {
	converter := &GeminiToOpenAIStreamConverter{}
	ctx := context.Background()

	// Simulate Gemini SSE stream
	streamData := `data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Hello"}]}}]}

data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":" world"}]}}]}

data: {"candidates":[{"index":0,"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":10,"totalTokenCount":15}}

`

	reader := strings.NewReader(streamData)
	var chunks []*dto.ChatCompletionStreamResponse

	chunkWriter := func(chunk any) error {
		streamChunk, ok := chunk.(*dto.ChatCompletionStreamResponse)
		if !ok {
			t.Fatalf("Expected *dto.ChatCompletionStreamResponse, got %T", chunk)
		}
		chunks = append(chunks, streamChunk)
		return nil
	}

	if err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter); err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	// Should have 4 chunks: role + "Hello" + " world" + final
	if len(chunks) != 4 {
		t.Fatalf("Chunk count = %d, want 4", len(chunks))
	}

	// Check first chunk (role)
	if chunks[0].Choices[0].Delta.Role != "assistant" {
		t.Errorf("First chunk role = %q, want %q", chunks[0].Choices[0].Delta.Role, "assistant")
	}

	// Check second chunk (text)
	if chunks[1].Choices[0].Delta.Content != "Hello" {
		t.Errorf("Second chunk content = %q, want %q", chunks[1].Choices[0].Delta.Content, "Hello")
	}

	// Check third chunk (text)
	if chunks[2].Choices[0].Delta.Content != " world" {
		t.Errorf("Third chunk content = %q, want %q", chunks[2].Choices[0].Delta.Content, " world")
	}

	// Check final chunk (finish reason + usage)
	if chunks[3].Choices[0].FinishReason == nil {
		t.Error("Final chunk missing finish reason")
	} else if *chunks[3].Choices[0].FinishReason != "stop" {
		t.Errorf("Final chunk finish reason = %q, want %q", *chunks[3].Choices[0].FinishReason, "stop")
	}

	if chunks[3].Usage == nil {
		t.Error("Final chunk missing usage")
	} else {
		if chunks[3].Usage.PromptTokens != 5 {
			t.Errorf("Usage PromptTokens = %d, want 5", chunks[3].Usage.PromptTokens)
		}
		if chunks[3].Usage.CompletionTokens != 10 {
			t.Errorf("Usage CompletionTokens = %d, want 10", chunks[3].Usage.CompletionTokens)
		}
	}
}

func TestGeminiToOpenAIStreamConverter_ToolCallStreaming(t *testing.T) {
	converter := &GeminiToOpenAIStreamConverter{}
	ctx := context.Background()

	// Simulate Gemini SSE stream with function call
	streamData := `data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"location":"NYC"}}}]}}],"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":12,"totalTokenCount":20}}

data: {"candidates":[{"index":0,"finishReason":"STOP"}]}

`

	reader := strings.NewReader(streamData)
	var chunks []*dto.ChatCompletionStreamResponse

	chunkWriter := func(chunk any) error {
		streamChunk, ok := chunk.(*dto.ChatCompletionStreamResponse)
		if !ok {
			t.Fatalf("Expected *dto.ChatCompletionStreamResponse, got %T", chunk)
		}
		chunks = append(chunks, streamChunk)
		return nil
	}

	if err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter); err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	// Should have 3 chunks: role + tool call + final
	if len(chunks) != 3 {
		t.Fatalf("Chunk count = %d, want 3", len(chunks))
	}

	// Find tool call chunk (skip role chunk)
	var toolCallChunk *dto.ChatCompletionStreamResponse
	for _, chunk := range chunks {
		if chunk.Choices[0].Delta.Role == "" && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			toolCallChunk = chunk
			break
		}
	}
	if toolCallChunk == nil {
		t.Fatal("Tool call chunk not found")
	}

	// Check tool call chunk
	if len(toolCallChunk.Choices[0].Delta.ToolCalls) != 1 {
		t.Fatalf("ToolCalls count = %d, want 1", len(toolCallChunk.Choices[0].Delta.ToolCalls))
	}

	toolCall := chunks[1].Choices[0].Delta.ToolCalls[0]
	if toolCall.Function.Name != "get_weather" {
		t.Errorf("Function name = %q, want %q", toolCall.Function.Name, "get_weather")
	}

	// Parse arguments
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		t.Fatalf("Failed to parse arguments: %v", err)
	}
	if args["location"] != "NYC" {
		t.Errorf("Location = %v, want %q", args["location"], "NYC")
	}
}

func TestGeminiToOpenAIStreamConverter_ThinkingStreaming(t *testing.T) {
	converter := &GeminiToOpenAIStreamConverter{}
	ctx := context.Background()

	// Simulate Gemini SSE stream with thinking
	streamData := `data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"thought":true,"text":"Let me think..."}]}}]}

data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Answer is 42"}]}}]}

data: {"candidates":[{"index":0,"finishReason":"STOP"}]}

`

	reader := strings.NewReader(streamData)
	var chunks []*dto.ChatCompletionStreamResponse

	chunkWriter := func(chunk any) error {
		streamChunk, ok := chunk.(*dto.ChatCompletionStreamResponse)
		if !ok {
			t.Fatalf("Expected *dto.ChatCompletionStreamResponse, got %T", chunk)
		}
		chunks = append(chunks, streamChunk)
		return nil
	}

	if err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter); err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	// Should have 4 chunks: role + thinking + text + final
	if len(chunks) != 4 {
		t.Fatalf("Chunk count = %d, want 4", len(chunks))
	}

	// Check thinking chunk
	if chunks[1].Choices[0].Delta.ReasoningContent == nil {
		t.Error("Thinking chunk missing reasoning_content")
	} else if *chunks[1].Choices[0].Delta.ReasoningContent != "Let me think..." {
		t.Errorf("ReasoningContent = %q, want %q", *chunks[1].Choices[0].Delta.ReasoningContent, "Let me think...")
	}

	// Check text chunk
	content, ok := chunks[2].Choices[0].Delta.Content.(string)
	if !ok {
		t.Fatalf("Text chunk content is not string, got %T", chunks[2].Choices[0].Delta.Content)
	}
	if content != "Answer is 42" {
		t.Errorf("Text chunk content = %q, want %q", content, "Answer is 42")
	}
}

func TestGeminiToOpenAIStreamConverter_SafetyBlock(t *testing.T) {
	converter := &GeminiToOpenAIStreamConverter{}
	ctx := context.Background()

	// Simulate Gemini safety block
	streamData := `data: {"promptFeedback":{"blockReason":"SAFETY"}}

`

	reader := strings.NewReader(streamData)
	chunkWriter := func(chunk any) error { return nil }

	err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter)
	if err == nil {
		t.Fatal("Expected error for safety block, got nil")
	}
	if !strings.Contains(err.Error(), "SAFETY") {
		t.Errorf("Error message should contain SAFETY, got %q", err.Error())
	}
}

func TestGeminiToOpenAIStreamConverter_ImageStreaming(t *testing.T) {
	converter := &GeminiToOpenAIStreamConverter{}
	ctx := context.Background()

	// Simulate Gemini SSE stream with inline data
	streamData := `data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"inlineData":{"mimeType":"image/png","data":"ABC123"}}]}}]}

`

	reader := strings.NewReader(streamData)
	var chunks []*dto.ChatCompletionStreamResponse

	chunkWriter := func(chunk any) error {
		streamChunk, ok := chunk.(*dto.ChatCompletionStreamResponse)
		if !ok {
			t.Fatalf("Expected *dto.ChatCompletionStreamResponse, got %T", chunk)
		}
		chunks = append(chunks, streamChunk)
		return nil
	}

	if err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter); err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	if len(chunks) != 3 {
		t.Fatalf("Chunk count = %d, want 3 (role + image + final)", len(chunks))
	}

	content, ok := chunks[1].Choices[0].Delta.Content.(string)
	if !ok {
		t.Fatalf("Content is not string, got %T", chunks[1].Choices[0].Delta.Content)
	}
	if !strings.Contains(content, "![image](data:image/png;base64,ABC123)") {
		t.Errorf("Content = %q, should contain image markdown", content)
	}
}

func TestGeminiToOpenAIStreamConverter_CodeStreaming(t *testing.T) {
	converter := &GeminiToOpenAIStreamConverter{}
	ctx := context.Background()

	// Simulate Gemini SSE stream with executable code
	streamData := `data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"executableCode":{"language":"python","code":"print(42)"}}]}}]}

data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"codeExecutionResult":{"outcome":"OUTCOME_OK","output":"42"}}]}}]}

`

	reader := strings.NewReader(streamData)
	var chunks []*dto.ChatCompletionStreamResponse

	chunkWriter := func(chunk any) error {
		streamChunk, ok := chunk.(*dto.ChatCompletionStreamResponse)
		if !ok {
			t.Fatalf("Expected *dto.ChatCompletionStreamResponse, got %T", chunk)
		}
		chunks = append(chunks, streamChunk)
		return nil
	}

	if err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter); err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	if len(chunks) != 4 {
		t.Fatalf("Chunk count = %d, want 4 (role + code + result + final)", len(chunks))
	}

	// Check code chunk
	codeContent, ok := chunks[1].Choices[0].Delta.Content.(string)
	if !ok {
		t.Fatalf("Code chunk content is not string, got %T", chunks[1].Choices[0].Delta.Content)
	}
	if !strings.Contains(codeContent, "```python") {
		t.Errorf("Code chunk should contain code block, got %q", codeContent)
	}

	// Check execution result chunk
	resultContent, ok := chunks[2].Choices[0].Delta.Content.(string)
	if !ok {
		t.Fatalf("Result chunk content is not string, got %T", chunks[2].Choices[0].Delta.Content)
	}
	if !strings.Contains(resultContent, "Execution OUTCOME_OK") {
		t.Errorf("Result chunk should contain execution outcome, got %q", resultContent)
	}
}

func TestGeminiToOpenAIStreamConverter_MultipleCandidates(t *testing.T) {
	converter := &GeminiToOpenAIStreamConverter{}
	ctx := context.Background()

	// Simulate Gemini SSE stream with multiple candidates
	streamData := `data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"First"}]}},{"index":1,"content":{"role":"model","parts":[{"text":"Second"}]}}]}

`

	reader := strings.NewReader(streamData)
	var chunks []*dto.ChatCompletionStreamResponse

	chunkWriter := func(chunk any) error {
		streamChunk, ok := chunk.(*dto.ChatCompletionStreamResponse)
		if !ok {
			t.Fatalf("Expected *dto.ChatCompletionStreamResponse, got %T", chunk)
		}
		chunks = append(chunks, streamChunk)
		return nil
	}

	if err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter); err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	// Should have 4 chunks: role + first candidate + second candidate + final
	if len(chunks) != 4 {
		t.Fatalf("Chunk count = %d, want 4", len(chunks))
	}

	// Find candidate chunks (skip role chunk)
	var candidateChunks []string
	for _, chunk := range chunks {
		if chunk.Choices[0].Delta.Role == "" && chunk.Choices[0].Delta.Content != nil {
			if content, ok := chunk.Choices[0].Delta.Content.(string); ok {
				candidateChunks = append(candidateChunks, content)
			}
		}
	}

	if len(candidateChunks) != 2 {
		t.Fatalf("Candidate chunks count = %d, want 2", len(candidateChunks))
	}

	if candidateChunks[0] != "First" {
		t.Errorf("First candidate content = %q, want %q", candidateChunks[0], "First")
	}

	if candidateChunks[1] != "Second" {
		t.Errorf("Second candidate content = %q, want %q", candidateChunks[1], "Second")
	}
}

func TestGeminiToOpenAIStreamConverter_FileDataStreaming(t *testing.T) {
	converter := &GeminiToOpenAIStreamConverter{}
	ctx := context.Background()

	// Simulate Gemini SSE stream with file data
	streamData := `data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"fileData":{"fileUri":"gs://my-bucket/document.pdf"}}]}}]}

`

	reader := strings.NewReader(streamData)
	var chunks []*dto.ChatCompletionStreamResponse

	chunkWriter := func(chunk any) error {
		streamChunk, ok := chunk.(*dto.ChatCompletionStreamResponse)
		if !ok {
			t.Fatalf("Expected *dto.ChatCompletionStreamResponse, got %T", chunk)
		}
		chunks = append(chunks, streamChunk)
		return nil
	}

	if err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter); err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	if len(chunks) != 3 {
		t.Fatalf("Chunk count = %d, want 3 (role + file + final)", len(chunks))
	}

	content, ok := chunks[1].Choices[0].Delta.Content.(string)
	if !ok {
		t.Fatalf("Content is not string, got %T", chunks[1].Choices[0].Delta.Content)
	}
	if !strings.Contains(content, "[file](gs://my-bucket/document.pdf)") {
		t.Errorf("Content = %q, should contain file link", content)
	}
}
