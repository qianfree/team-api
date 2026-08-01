package oai_gemini

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

func TestOpenAIToGeminiRequestConverter_Metadata(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}

	if converter.ID() != relayconvert.ConverterOpenAIChatToGeminiContent {
		t.Errorf("ID() = %q, want %q", converter.ID(), relayconvert.ConverterOpenAIChatToGeminiContent)
	}

	if converter.From() != types.RelayFormatOpenAI {
		t.Errorf("From() = %q, want %q", converter.From(), types.RelayFormatOpenAI)
	}

	if converter.To() != types.RelayFormatGemini {
		t.Errorf("To() = %q, want %q", converter.To(), types.RelayFormatGemini)
	}

	if converter.Quality() != relayconvert.RequestConverterQualityGood {
		t.Errorf("Quality() = %q, want %q", converter.Quality(), relayconvert.RequestConverterQualityGood)
	}
}

func TestOpenAIToGeminiRequestConverter_BasicConversion(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	temp := 0.7
	topP := 0.9
	maxTokens := 1024

	openaiReq := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello, how are you?"},
		},
		Temperature: &temp,
		TopP:        &topP,
		MaxTokens:   &maxTokens,
	}

	result, err := converter.ConvertRequest(ctx, nil, openaiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	geminiReq, ok := result.(*dto.GeminiChatRequest)
	if !ok {
		t.Fatalf("Expected *dto.GeminiChatRequest, got %T", result)
	}

	// Check generation config
	if geminiReq.GenerationConfig == nil {
		t.Fatal("GenerationConfig is nil")
	}
	if geminiReq.GenerationConfig.Temperature == nil || *geminiReq.GenerationConfig.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", geminiReq.GenerationConfig.Temperature)
	}
	if geminiReq.GenerationConfig.TopP == nil || *geminiReq.GenerationConfig.TopP != 0.9 {
		t.Errorf("TopP = %v, want 0.9", geminiReq.GenerationConfig.TopP)
	}
	if geminiReq.GenerationConfig.MaxOutputTokens == nil || *geminiReq.GenerationConfig.MaxOutputTokens != 1024 {
		t.Errorf("MaxOutputTokens = %v, want 1024", geminiReq.GenerationConfig.MaxOutputTokens)
	}

	// Check system instruction
	if geminiReq.SystemInstruction == nil {
		t.Fatal("SystemInstruction is nil")
	}
	if len(geminiReq.SystemInstruction.Parts) != 1 {
		t.Fatalf("SystemInstruction parts count = %d, want 1", len(geminiReq.SystemInstruction.Parts))
	}
	if geminiReq.SystemInstruction.Parts[0].Text != "You are a helpful assistant." {
		t.Errorf("SystemInstruction text = %q, want %q", geminiReq.SystemInstruction.Parts[0].Text, "You are a helpful assistant.")
	}

	// Check contents
	if len(geminiReq.Contents) != 1 {
		t.Fatalf("Contents count = %d, want 1", len(geminiReq.Contents))
	}
	if geminiReq.Contents[0].Role != "user" {
		t.Errorf("First content role = %q, want %q", geminiReq.Contents[0].Role, "user")
	}
	if len(geminiReq.Contents[0].Parts) != 1 {
		t.Fatalf("First content parts count = %d, want 1", len(geminiReq.Contents[0].Parts))
	}
	if geminiReq.Contents[0].Parts[0].Text != "Hello, how are you?" {
		t.Errorf("First content text = %q, want %q", geminiReq.Contents[0].Parts[0].Text, "Hello, how are you?")
	}

	// Check safety settings
	if len(geminiReq.SafetySettings) != 4 {
		t.Errorf("SafetySettings count = %d, want 4", len(geminiReq.SafetySettings))
	}
}

func TestOpenAIToGeminiRequestConverter_ToolCalls(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	openaiReq := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []dto.ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Function: dto.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"San Francisco","unit":"celsius"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				Content:    `{"temperature":18,"condition":"sunny"}`,
				ToolCallID: "call_123",
			},
		},
		Tools: []dto.Tool{
			{
				Type: "function",
				Function: dto.FunctionDef{
					Name:        "get_weather",
					Description: "Get weather for a location",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{"type": "string"},
							"unit":     map[string]any{"type": "string", "enum": []string{"celsius", "fahrenheit"}},
						},
						"required": []string{"location"},
					},
				},
			},
		},
	}

	result, err := converter.ConvertRequest(ctx, nil, openaiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	geminiReq, ok := result.(*dto.GeminiChatRequest)
	if !ok {
		t.Fatalf("Expected *dto.GeminiChatRequest, got %T", result)
	}

	// Check tools
	if geminiReq.Tools == nil {
		t.Fatal("Tools is nil")
	}
	var tools []geminiTool
	if err := json.Unmarshal(geminiReq.Tools, &tools); err != nil {
		t.Fatalf("Failed to unmarshal tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("Tools count = %d, want 1", len(tools))
	}
	if len(tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("FunctionDeclarations count = %d, want 1", len(tools[0].FunctionDeclarations))
	}
	if tools[0].FunctionDeclarations[0].Name != "get_weather" {
		t.Errorf("Function name = %q, want %q", tools[0].FunctionDeclarations[0].Name, "get_weather")
	}

	// Check contents structure: user -> model (with functionCall) -> user (with functionResponse)
	if len(geminiReq.Contents) != 3 {
		t.Fatalf("Contents count = %d, want 3", len(geminiReq.Contents))
	}

	// First: user message
	if geminiReq.Contents[0].Role != "user" {
		t.Errorf("First content role = %q, want %q", geminiReq.Contents[0].Role, "user")
	}

	// Second: assistant with tool call
	if geminiReq.Contents[1].Role != "model" {
		t.Errorf("Second content role = %q, want %q", geminiReq.Contents[1].Role, "model")
	}
	if len(geminiReq.Contents[1].Parts) == 0 {
		t.Fatal("Second content has no parts")
	}
	foundFunctionCall := false
	for _, part := range geminiReq.Contents[1].Parts {
		if part.FunctionCall != nil {
			foundFunctionCall = true
			if part.FunctionCall.FunctionName != "get_weather" {
				t.Errorf("FunctionCall name = %q, want %q", part.FunctionCall.FunctionName, "get_weather")
			}
			if part.FunctionCall.Arguments == nil {
				t.Error("FunctionCall arguments is nil")
			}
		}
	}
	if !foundFunctionCall {
		t.Error("No function call found in assistant message")
	}

	// Third: user with function response
	if geminiReq.Contents[2].Role != "user" {
		t.Errorf("Third content role = %q, want %q", geminiReq.Contents[2].Role, "user")
	}
	foundFunctionResponse := false
	for _, part := range geminiReq.Contents[2].Parts {
		if part.FunctionResponse != nil {
			foundFunctionResponse = true
			if part.FunctionResponse.Name != "get_weather" {
				t.Errorf("FunctionResponse name = %q, want %q", part.FunctionResponse.Name, "get_weather")
			}
		}
	}
	if !foundFunctionResponse {
		t.Error("No function response found in tool message")
	}
}

func TestOpenAIToGeminiRequestConverter_MultimodalContent(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	openaiReq := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "text", "text": "What's in this image?"},
					map[string]any{
						"type": "image_url",
						"image_url": map[string]any{
							"url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
						},
					},
				},
			},
		},
	}

	result, err := converter.ConvertRequest(ctx, nil, openaiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	geminiReq, ok := result.(*dto.GeminiChatRequest)
	if !ok {
		t.Fatalf("Expected *dto.GeminiChatRequest, got %T", result)
	}

	if len(geminiReq.Contents) != 1 {
		t.Fatalf("Contents count = %d, want 1", len(geminiReq.Contents))
	}

	parts := geminiReq.Contents[0].Parts
	if len(parts) != 2 {
		t.Fatalf("Parts count = %d, want 2", len(parts))
	}

	// First part: text
	if parts[0].Text != "What's in this image?" {
		t.Errorf("First part text = %q, want %q", parts[0].Text, "What's in this image?")
	}

	// Second part: image inline data
	if parts[1].InlineData == nil {
		t.Fatal("Second part InlineData is nil")
	}
	if parts[1].InlineData.MimeType != "image/png" {
		t.Errorf("InlineData MimeType = %q, want %q", parts[1].InlineData.MimeType, "image/png")
	}
	if parts[1].InlineData.Data == "" {
		t.Error("InlineData Data is empty")
	}
}

func TestOpenAIToGeminiRequestConverter_ReasoningEffort(t *testing.T) {
	tests := []struct {
		name   string
		effort string
		want   struct {
			budget int
			level  string
		}
	}{
		{
			name:   "low effort",
			effort: "low",
			want:   struct{ budget int; level string }{budget: 1024, level: "LOW"},
		},
		{
			name:   "medium effort",
			effort: "medium",
			want:   struct{ budget int; level string }{budget: 8192, level: "MEDIUM"},
		},
		{
			name:   "high effort",
			effort: "high",
			want:   struct{ budget int; level string }{budget: 32768, level: "HIGH"},
		},
		{
			name:   "unknown effort defaults to medium",
			effort: "unknown",
			want:   struct{ budget int; level string }{budget: 8192, level: "MEDIUM"},
		},
	}

	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openaiReq := &dto.GeneralOpenAIRequest{
				Messages:        []dto.Message{{Role: "user", Content: "Test"}},
				ReasoningEffort: tt.effort,
			}

			result, err := converter.ConvertRequest(ctx, nil, openaiReq)
			if err != nil {
				t.Fatalf("ConvertRequest failed: %v", err)
			}

			geminiReq, ok := result.(*dto.GeminiChatRequest)
			if !ok {
				t.Fatalf("Expected *dto.GeminiChatRequest, got %T", result)
			}

			if geminiReq.GenerationConfig == nil || geminiReq.GenerationConfig.ThinkingConfig == nil {
				t.Fatal("ThinkingConfig is nil")
			}

			thinkingConfig := geminiReq.GenerationConfig.ThinkingConfig
			if !thinkingConfig.IncludeThoughts {
				t.Error("IncludeThoughts should be true")
			}
			if thinkingConfig.ThoughtBudget == nil || *thinkingConfig.ThoughtBudget != tt.want.budget {
				t.Errorf("ThoughtBudget = %v, want %d", thinkingConfig.ThoughtBudget, tt.want.budget)
			}
			if thinkingConfig.ThinkingLevel != tt.want.level {
				t.Errorf("ThinkingLevel = %q, want %q", thinkingConfig.ThinkingLevel, tt.want.level)
			}
		})
	}
}

func TestOpenAIToGeminiRequestConverter_ResponseFormat(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"age":  map[string]any{"type": "integer"},
		},
		"required": []string{"name"},
	}

	openaiReq := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "user", Content: "Test"}},
		ResponseFormat: &dto.ResponseFormat{
			Type:       "json_schema",
			JSONSchema: schema,
		},
	}

	result, err := converter.ConvertRequest(ctx, nil, openaiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	geminiReq, ok := result.(*dto.GeminiChatRequest)
	if !ok {
		t.Fatalf("Expected *dto.GeminiChatRequest, got %T", result)
	}

	if geminiReq.GenerationConfig == nil {
		t.Fatal("GenerationConfig is nil")
	}

	if geminiReq.GenerationConfig.ResponseMimeType != "application/json" {
		t.Errorf("ResponseMimeType = %q, want %q", geminiReq.GenerationConfig.ResponseMimeType, "application/json")
	}

	if geminiReq.GenerationConfig.ResponseSchema == nil {
		t.Fatal("ResponseSchema is nil")
	}

	// Check that schema types are converted to uppercase
	schemaMap, ok := geminiReq.GenerationConfig.ResponseSchema.(map[string]any)
	if !ok {
		t.Fatalf("ResponseSchema is not map[string]any, got %T", geminiReq.GenerationConfig.ResponseSchema)
	}

	if schemaMap["type"] != "OBJECT" {
		t.Errorf("Schema type = %v, want %q", schemaMap["type"], "OBJECT")
	}

	props, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		t.Fatal("Schema properties not found or wrong type")
	}

	nameSchema, ok := props["name"].(map[string]any)
	if !ok {
		t.Fatal("name property not found")
	}
	if nameSchema["type"] != "STRING" {
		t.Errorf("name type = %v, want %q", nameSchema["type"], "STRING")
	}

	ageSchema, ok := props["age"].(map[string]any)
	if !ok {
		t.Fatal("age property not found")
	}
	if ageSchema["type"] != "INTEGER" {
		t.Errorf("age type = %v, want %q", ageSchema["type"], "INTEGER")
	}
}

func TestOpenAIToGeminiRequestConverter_StopSequences(t *testing.T) {
	tests := []struct {
		name string
		stop any
		want []string
	}{
		{
			name: "string stop",
			stop: "STOP",
			want: []string{"STOP"},
		},
		{
			name: "array of strings",
			stop: []string{"END", "DONE", "STOP"},
			want: []string{"END", "DONE", "STOP"},
		},
		{
			name: "more than 5 stops are trimmed",
			stop: []string{"A", "B", "C", "D", "E", "F", "G"},
			want: []string{"A", "B", "C", "D", "E"},
		},
		{
			name: "nil stop",
			stop: nil,
			want: nil,
		},
	}

	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openaiReq := &dto.GeneralOpenAIRequest{
				Messages: []dto.Message{{Role: "user", Content: "Test"}},
				Stop:     tt.stop,
			}

			result, err := converter.ConvertRequest(ctx, nil, openaiReq)
			if err != nil {
				t.Fatalf("ConvertRequest failed: %v", err)
			}

			geminiReq, ok := result.(*dto.GeminiChatRequest)
			if !ok {
				t.Fatalf("Expected *dto.GeminiChatRequest, got %T", result)
			}

			if geminiReq.GenerationConfig == nil {
				t.Fatal("GenerationConfig is nil")
			}

			got := geminiReq.GenerationConfig.StopSequences
			if len(got) != len(tt.want) {
				t.Errorf("StopSequences length = %d, want %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("StopSequences[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestOpenAIToGeminiRequestConverter_ToolChoice(t *testing.T) {
	tests := []struct {
		name       string
		toolChoice any
		wantMode   string
	}{
		{
			name:       "auto",
			toolChoice: "auto",
			wantMode:   "AUTO",
		},
		{
			name:       "none",
			toolChoice: "none",
			wantMode:   "NONE",
		},
		{
			name:       "required",
			toolChoice: "required",
			wantMode:   "ANY",
		},
		{
			name: "specific function",
			toolChoice: map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": "get_weather",
				},
			},
			wantMode: "ANY",
		},
	}

	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openaiReq := &dto.GeneralOpenAIRequest{
				Messages: []dto.Message{{Role: "user", Content: "Test"}},
				Tools: []dto.Tool{
					{
						Type: "function",
						Function: dto.FunctionDef{
							Name:        "get_weather",
							Description: "Get weather",
						},
					},
				},
				ToolChoice: tt.toolChoice,
			}

			result, err := converter.ConvertRequest(ctx, nil, openaiReq)
			if err != nil {
				t.Fatalf("ConvertRequest failed: %v", err)
			}

			geminiReq, ok := result.(*dto.GeminiChatRequest)
			if !ok {
				t.Fatalf("Expected *dto.GeminiChatRequest, got %T", result)
			}

			if geminiReq.ToolConfig == nil {
				t.Fatal("ToolConfig is nil")
			}

			toolConfig, ok := geminiReq.ToolConfig.(map[string]any)
			if !ok {
				t.Fatalf("ToolConfig is not map[string]any, got %T", geminiReq.ToolConfig)
			}

			funcCallingConfig, ok := toolConfig["functionCallingConfig"].(map[string]any)
			if !ok {
				t.Fatal("functionCallingConfig not found")
			}

			mode, ok := funcCallingConfig["mode"].(string)
			if !ok {
				t.Fatal("mode not found")
			}

			if mode != tt.wantMode {
				t.Errorf("mode = %q, want %q", mode, tt.wantMode)
			}

			// Check specific function name if applicable
			if tt.name == "specific function" {
				allowedNames, ok := funcCallingConfig["allowedFunctionNames"].([]string)
				if !ok {
					t.Fatal("allowedFunctionNames not found")
				}
				if len(allowedNames) != 1 || allowedNames[0] != "get_weather" {
					t.Errorf("allowedFunctionNames = %v, want [get_weather]", allowedNames)
				}
			}
		})
	}
}

func TestOpenAIToGeminiRequestConverter_AssistantWithReasoning(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	reasoning := "Let me think about this carefully..."
	openaiReq := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "user", Content: "Solve this problem"},
			{Role: "assistant", Content: "The answer is 42", ReasoningContent: &reasoning},
		},
	}

	result, err := converter.ConvertRequest(ctx, nil, openaiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	geminiReq, ok := result.(*dto.GeminiChatRequest)
	if !ok {
		t.Fatalf("Expected *dto.GeminiChatRequest, got %T", result)
	}

	if len(geminiReq.Contents) != 2 {
		t.Fatalf("Contents count = %d, want 2", len(geminiReq.Contents))
	}

	// Check assistant message
	assistantContent := geminiReq.Contents[1]
	if assistantContent.Role != "model" {
		t.Errorf("Assistant role = %q, want %q", assistantContent.Role, "model")
	}

	// Should have 2 parts: regular text + thought
	if len(assistantContent.Parts) != 2 {
		t.Fatalf("Assistant parts count = %d, want 2", len(assistantContent.Parts))
	}

	// First part: regular text
	if assistantContent.Parts[0].Text != "The answer is 42" {
		t.Errorf("First part text = %q, want %q", assistantContent.Parts[0].Text, "The answer is 42")
	}

	// Second part: thought
	if assistantContent.Parts[1].Text != reasoning {
		t.Errorf("Second part text = %q, want %q", assistantContent.Parts[1].Text, reasoning)
	}
	if assistantContent.Parts[1].Thought == nil || !*assistantContent.Parts[1].Thought {
		t.Error("Second part should have Thought = true")
	}
}

func TestOpenAIToGeminiRequestConverter_InvalidRequestType(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	_, err := converter.ConvertRequest(ctx, nil, "not a request")
	if err == nil {
		t.Fatal("Expected error for invalid request type, got nil")
	}
}
