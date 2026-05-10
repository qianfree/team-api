package gemini

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

func makeTestInfo() *common.RelayInfo {
	return &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{
			UpstreamModelName: "gemini-2.5-flash",
		},
		InboundFormat: "openai",
	}
}

func TestConvertOpenAIToGemini_SimpleChat(t *testing.T) {
	info := makeTestInfo()

	openaiReq := dto.GeneralOpenAIRequest{
		Model: "gemini-2.5-flash",
		Messages: []dto.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
	}

	body, _ := json.Marshal(openaiReq)
	reader, err := ConvertOpenAIToGemini(body, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var geminiReq dto.GeminiChatRequest
	data, _ := io.ReadAll(reader)
	if err := json.Unmarshal(data, &geminiReq); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if geminiReq.SystemInstruction == nil {
		t.Fatal("expected SystemInstruction to be set")
	}
	if len(geminiReq.SystemInstruction.Parts) != 1 || geminiReq.SystemInstruction.Parts[0].Text != "You are helpful." {
		t.Errorf("expected system instruction 'You are helpful.', got %v", geminiReq.SystemInstruction)
	}

	if len(geminiReq.Contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(geminiReq.Contents))
	}
	if geminiReq.Contents[0].Role != "user" {
		t.Errorf("expected first content role 'user', got %s", geminiReq.Contents[0].Role)
	}
	if geminiReq.Contents[1].Role != "model" {
		t.Errorf("expected second content role 'model', got %s", geminiReq.Contents[1].Role)
	}
	if geminiReq.Contents[2].Role != "user" {
		t.Errorf("expected third content role 'user', got %s", geminiReq.Contents[2].Role)
	}
}

func TestConvertOpenAIToGemini_ToolUse(t *testing.T) {
	info := makeTestInfo()

	openaiReq := dto.GeneralOpenAIRequest{
		Model: "gemini-2.5-flash",
		Messages: []dto.Message{
			{Role: "user", Content: "What's the weather?"},
			{
				Role: "assistant",
				ToolCalls: []dto.ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Function: dto.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"NYC"}`,
						},
					},
				},
			},
			{Role: "tool", ToolCallID: "call_123", Content: "Sunny, 72°F"},
		},
	}

	body, _ := json.Marshal(openaiReq)
	reader, err := ConvertOpenAIToGemini(body, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var geminiReq dto.GeminiChatRequest
	data, _ := io.ReadAll(reader)
	if err := json.Unmarshal(data, &geminiReq); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(geminiReq.Contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(geminiReq.Contents))
	}

	assistantContent := geminiReq.Contents[1]
	if assistantContent.Role != "model" {
		t.Errorf("expected role 'model', got %s", assistantContent.Role)
	}
	hasFunctionCall := false
	for _, part := range assistantContent.Parts {
		if part.FunctionCall != nil {
			hasFunctionCall = true
			if part.FunctionCall.FunctionName != "get_weather" {
				t.Errorf("expected function name 'get_weather', got %s", part.FunctionCall.FunctionName)
			}
		}
	}
	if !hasFunctionCall {
		t.Error("expected function call in assistant content")
	}

	toolContent := geminiReq.Contents[2]
	if toolContent.Role != "user" {
		t.Errorf("expected role 'user' for tool result, got %s", toolContent.Role)
	}
	hasFunctionResponse := false
	for _, part := range toolContent.Parts {
		if part.FunctionResponse != nil {
			hasFunctionResponse = true
		}
	}
	if !hasFunctionResponse {
		t.Error("expected function response in tool content")
	}
}

func TestConvertOpenAIToGemini_GenerationConfig(t *testing.T) {
	info := makeTestInfo()

	maxTokens := 1024
	temp := 0.7
	presence := 0.5
	frequency := 0.3
	n := 2
	openaiReq := dto.GeneralOpenAIRequest{
		Model:            "gemini-2.5-flash",
		MaxTokens:        &maxTokens,
		Temperature:      &temp,
		PresencePenalty:  &presence,
		FrequencyPenalty: &frequency,
		N:                &n,
		Messages: []dto.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(openaiReq)
	reader, err := ConvertOpenAIToGemini(body, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var geminiReq dto.GeminiChatRequest
	data, _ := io.ReadAll(reader)
	if err := json.Unmarshal(data, &geminiReq); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if geminiReq.GenerationConfig == nil {
		t.Fatal("expected GenerationConfig to be set")
	}
	if geminiReq.GenerationConfig.MaxOutputTokens == nil || *geminiReq.GenerationConfig.MaxOutputTokens != 1024 {
		t.Errorf("expected MaxOutputTokens 1024, got %v", geminiReq.GenerationConfig.MaxOutputTokens)
	}
	if geminiReq.GenerationConfig.Temperature == nil || *geminiReq.GenerationConfig.Temperature != 0.7 {
		t.Errorf("expected Temperature 0.7, got %v", geminiReq.GenerationConfig.Temperature)
	}
	if geminiReq.GenerationConfig.PresencePenalty == nil || *geminiReq.GenerationConfig.PresencePenalty != 0.5 {
		t.Errorf("expected PresencePenalty 0.5, got %v", geminiReq.GenerationConfig.PresencePenalty)
	}
	if geminiReq.GenerationConfig.FrequencyPenalty == nil || *geminiReq.GenerationConfig.FrequencyPenalty != 0.3 {
		t.Errorf("expected FrequencyPenalty 0.3, got %v", geminiReq.GenerationConfig.FrequencyPenalty)
	}
	if geminiReq.GenerationConfig.CandidateCount == nil || *geminiReq.GenerationConfig.CandidateCount != 2 {
		t.Errorf("expected CandidateCount 2, got %v", geminiReq.GenerationConfig.CandidateCount)
	}
}

func TestConvertOpenAIToGemini_SafetySettings(t *testing.T) {
	info := makeTestInfo()

	openaiReq := dto.GeneralOpenAIRequest{
		Model: "gemini-2.5-flash",
		Messages: []dto.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(openaiReq)
	reader, err := ConvertOpenAIToGemini(body, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var geminiReq dto.GeminiChatRequest
	data, _ := io.ReadAll(reader)
	if err := json.Unmarshal(data, &geminiReq); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(geminiReq.SafetySettings) == 0 {
		t.Fatal("expected safety settings to be set")
	}
}

func TestConvertOpenAIToGemini_ReasoningEffort(t *testing.T) {
	info := makeTestInfo()

	openaiReq := dto.GeneralOpenAIRequest{
		Model:           "gemini-2.5-flash",
		ReasoningEffort: "high",
		Messages: []dto.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(openaiReq)
	reader, err := ConvertOpenAIToGemini(body, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var geminiReq dto.GeminiChatRequest
	data, _ := io.ReadAll(reader)
	if err := json.Unmarshal(data, &geminiReq); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if geminiReq.GenerationConfig.ThinkingConfig == nil {
		t.Fatal("expected ThinkingConfig to be set")
	}
	if !geminiReq.GenerationConfig.ThinkingConfig.IncludeThoughts {
		t.Error("expected IncludeThoughts to be true")
	}
	if geminiReq.GenerationConfig.ThinkingConfig.ThoughtBudget == nil || *geminiReq.GenerationConfig.ThinkingConfig.ThoughtBudget != 32768 {
		t.Errorf("expected ThoughtBudget 32768, got %v", geminiReq.GenerationConfig.ThinkingConfig.ThoughtBudget)
	}
	if geminiReq.GenerationConfig.ThinkingConfig.ThinkingLevel != "HIGH" {
		t.Errorf("expected ThinkingLevel HIGH, got %s", geminiReq.GenerationConfig.ThinkingConfig.ThinkingLevel)
	}
}

func TestConvertOpenAIToGemini_ResponseFormat(t *testing.T) {
	info := makeTestInfo()

	openaiReq := dto.GeneralOpenAIRequest{
		Model: "gemini-2.5-flash",
		ResponseFormat: &dto.ResponseFormat{
			Type: "json_schema",
			JSONSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age":  map[string]any{"type": "integer"},
					"tags": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
				"required": []any{"name"},
			},
		},
		Messages: []dto.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(openaiReq)
	reader, err := ConvertOpenAIToGemini(body, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var geminiReq dto.GeminiChatRequest
	data, _ := io.ReadAll(reader)
	if err := json.Unmarshal(data, &geminiReq); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if geminiReq.GenerationConfig.ResponseMimeType != "application/json" {
		t.Errorf("expected ResponseMimeType 'application/json', got %s", geminiReq.GenerationConfig.ResponseMimeType)
	}

	// Verify schema types are converted to uppercase
	schemaMap, ok := geminiReq.GenerationConfig.ResponseSchema.(map[string]any)
	if !ok {
		t.Fatalf("expected ResponseSchema to be a map, got %T", geminiReq.GenerationConfig.ResponseSchema)
	}
	if schemaMap["type"] != "OBJECT" {
		t.Errorf("expected top-level type 'OBJECT', got %v", schemaMap["type"])
	}

	props, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties to be a map, got %T", schemaMap["properties"])
	}
	nameField, ok := props["name"].(map[string]any)
	if !ok {
		t.Fatalf("expected name field to be a map")
	}
	if nameField["type"] != "STRING" {
		t.Errorf("expected name type 'STRING', got %v", nameField["type"])
	}

	ageField, ok := props["age"].(map[string]any)
	if !ok {
		t.Fatalf("expected age field to be a map")
	}
	if ageField["type"] != "INTEGER" {
		t.Errorf("expected age type 'INTEGER', got %v", ageField["type"])
	}

	tagsField, ok := props["tags"].(map[string]any)
	if !ok {
		t.Fatalf("expected tags field to be a map")
	}
	if tagsField["type"] != "ARRAY" {
		t.Errorf("expected tags type 'ARRAY', got %v", tagsField["type"])
	}
	itemsField, ok := tagsField["items"].(map[string]any)
	if !ok {
		t.Fatalf("expected items to be a map")
	}
	if itemsField["type"] != "STRING" {
		t.Errorf("expected items type 'STRING', got %v", itemsField["type"])
	}

	// Verify other fields are preserved
	required, ok := schemaMap["required"].([]any)
	if !ok || len(required) != 1 || required[0] != "name" {
		t.Errorf("expected required to be preserved as ['name'], got %v", schemaMap["required"])
	}
}

func TestConvertOpenAIToGemini_JsonSchemaWrapper(t *testing.T) {
	info := makeTestInfo()

	openaiReq := dto.GeneralOpenAIRequest{
		Model: "gemini-2.5-flash",
		ResponseFormat: &dto.ResponseFormat{
			Type: "json_schema",
			JSONSchema: map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name": "person",
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
		Messages: []dto.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(openaiReq)
	reader, err := ConvertOpenAIToGemini(body, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var geminiReq dto.GeminiChatRequest
	data, _ := io.ReadAll(reader)
	if err := json.Unmarshal(data, &geminiReq); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	schemaMap, ok := geminiReq.GenerationConfig.ResponseSchema.(map[string]any)
	if !ok {
		t.Fatalf("expected ResponseSchema to be a map")
	}
	if schemaMap["type"] != "OBJECT" {
		t.Errorf("expected unwrapped schema type 'OBJECT', got %v", schemaMap["type"])
	}
}

func TestGeminiFinishReasonToOpenAI(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"STOP", "stop"},
		{"MAX_TOKENS", "length"},
		{"SAFETY", "content_filter"},
		{"RECITATION", "content_filter"},
		{"OTHER", "content_filter"},
		{"BLOCKLIST", "content_filter"},
		{"PROHIBITED", "content_filter"},
		{"SPII", "content_filter"},
		{"MALFORMED_FUNCTION_CALL", "tool_calls"},
		{"TOOL_CALLS", "tool_calls"},
		{"LANGUAGE", "content_filter"},
		{"IMAGE_SAFETY", "content_filter"},
		{"IMAGE_PROHIBITED_CONTENT", "content_filter"},
		{"IMAGE_OTHER", "content_filter"},
		{"NO_IMAGE", "content_filter"},
		{"IMAGE_RECITATION", "content_filter"},
		{"UNEXPECTED_TOOL_CALL", "tool_calls"},
		{"TOO_MANY_TOOL_CALLS", "tool_calls"},
		{"MISSING_THOUGHT_SIGNATURE", "stop"},
		{"MALFORMED_RESPONSE", "stop"},
		{"FINISH_REASON_UNSPECIFIED", "stop"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		result := common.GeminiFinishReasonToOpenAI(tt.input)
		if result != tt.expected {
			t.Errorf("GeminiFinishReasonToOpenAI(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGeminiFinishReasonToClaude(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"STOP", "end_turn"},
		{"MAX_TOKENS", "max_tokens"},
		{"SAFETY", "refusal"},
		{"RECITATION", "refusal"},
		{"LANGUAGE", "refusal"},
		{"IMAGE_SAFETY", "refusal"},
		{"IMAGE_RECITATION", "refusal"},
		{"MALFORMED_FUNCTION_CALL", "tool_use"},
		{"UNEXPECTED_TOOL_CALL", "tool_use"},
		{"TOO_MANY_TOOL_CALLS", "tool_use"},
		{"MISSING_THOUGHT_SIGNATURE", "end_turn"},
		{"MALFORMED_RESPONSE", "end_turn"},
		{"FINISH_REASON_UNSPECIFIED", "end_turn"},
		{"SPII", "refusal"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		result := common.GeminiFinishReasonToClaude(tt.input)
		if result != tt.expected {
			t.Errorf("GeminiFinishReasonToClaude(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestO2gMapSchemaType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"string", "STRING"},
		{"number", "NUMBER"},
		{"integer", "INTEGER"},
		{"boolean", "BOOLEAN"},
		{"object", "OBJECT"},
		{"array", "ARRAY"},
		{"null", "NULL"},
		{"custom", "custom"},
	}
	for _, tt := range tests {
		result := o2gMapSchemaType(tt.input)
		if result != tt.expected {
			t.Errorf("o2gMapSchemaType(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestO2gConvertReasoningEffort(t *testing.T) {
	tests := []struct {
		effort     string
		wantBudget int
		wantLevel  string
	}{
		{"low", 1024, "LOW"},
		{"medium", 8192, "MEDIUM"},
		{"high", 32768, "HIGH"},
		{"unknown", 8192, "MEDIUM"},
	}
	for _, tt := range tests {
		tc := o2gConvertReasoningEffort(tt.effort)
		if !tc.IncludeThoughts {
			t.Errorf("IncludeThoughts should be true for %q", tt.effort)
		}
		if tc.ThoughtBudget == nil || *tc.ThoughtBudget != tt.wantBudget {
			t.Errorf("effort %q: budget = %v, want %d", tt.effort, tc.ThoughtBudget, tt.wantBudget)
		}
		if tc.ThinkingLevel != tt.wantLevel {
			t.Errorf("effort %q: level = %q, want %q", tt.effort, tc.ThinkingLevel, tt.wantLevel)
		}
	}
}

func TestParseGeminiError(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantCode    int
		wantStatus  string
		wantMessage string
	}{
		{
			name:        "standard RPC error",
			body:        `{"error":{"code":400,"message":"Invalid argument","status":"INVALID_ARGUMENT"}}`,
			wantCode:    400,
			wantStatus:  "INVALID_ARGUMENT",
			wantMessage: "Invalid argument",
		},
		{
			name:        "auth error",
			body:        `{"error":{"code":401,"message":"API key not valid","status":"UNAUTHENTICATED"}}`,
			wantCode:    401,
			wantStatus:  "UNAUTHENTICATED",
			wantMessage: "API key not valid",
		},
		{
			name:        "non-RPC body",
			body:        `plain text error`,
			wantCode:    0,
			wantStatus:  "",
			wantMessage: "plain text error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, status, message := parseGeminiError([]byte(tt.body))
			if code != tt.wantCode {
				t.Errorf("code = %d, want %d", code, tt.wantCode)
			}
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
			if message != tt.wantMessage {
				t.Errorf("message = %q, want %q", message, tt.wantMessage)
			}
		})
	}
}

func TestGeminiStatusToOpenAIType(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"UNAUTHENTICATED", "authentication_error"},
		{"PERMISSION_DENIED", "permission_error"},
		{"INVALID_ARGUMENT", "invalid_request_error"},
		{"NOT_FOUND", "invalid_request_error"},
		{"RESOURCE_EXHAUSTED", "rate_limit_error"},
		{"RATE_LIMIT_EXCEEDED", "rate_limit_error"},
		{"INTERNAL", "internal_error"},
		{"UNAVAILABLE", "server_error"},
		{"DEADLINE_EXCEEDED", "timeout_error"},
		{"UNKNOWN_STATUS", "api_error"},
	}
	for _, tt := range tests {
		result := geminiStatusToOpenAIType(tt.status)
		if result != tt.expected {
			t.Errorf("geminiStatusToOpenAIType(%q) = %q, want %q", tt.status, result, tt.expected)
		}
	}
}

func TestConvertOpenAIToGemini_Logprobs(t *testing.T) {
	info := makeTestInfo()
	logprobs := true
	topLogprobs := 5

	openaiReq := dto.GeneralOpenAIRequest{
		Model:       "gemini-2.5-flash",
		LogProbs:    &logprobs,
		TopLogProbs: &topLogprobs,
		Messages: []dto.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(openaiReq)
	reader, err := ConvertOpenAIToGemini(body, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var geminiReq dto.GeminiChatRequest
	data, _ := io.ReadAll(reader)
	if err := json.Unmarshal(data, &geminiReq); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if geminiReq.GenerationConfig.ResponseLogprobs == nil || !*geminiReq.GenerationConfig.ResponseLogprobs {
		t.Error("expected ResponseLogprobs to be true")
	}
	if geminiReq.GenerationConfig.Logprobs == nil || *geminiReq.GenerationConfig.Logprobs != 5 {
		t.Errorf("expected Logprobs 5, got %v", geminiReq.GenerationConfig.Logprobs)
	}
}

func TestConvertOpenAIToGemini_ReasoningContentPassthrough(t *testing.T) {
	info := makeTestInfo()
	thinking := "Let me think about this..."

	openaiReq := dto.GeneralOpenAIRequest{
		Model: "gemini-2.5-flash",
		Messages: []dto.Message{
			{
				Role:             "assistant",
				ReasoningContent: &thinking,
			},
			{Role: "user", Content: "Continue"},
		},
	}

	body, _ := json.Marshal(openaiReq)
	reader, err := ConvertOpenAIToGemini(body, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var geminiReq dto.GeminiChatRequest
	data, _ := io.ReadAll(reader)
	if err := json.Unmarshal(data, &geminiReq); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// First content should be model with thought part
	if len(geminiReq.Contents) < 1 {
		t.Fatal("expected at least 1 content")
	}
	assistantContent := geminiReq.Contents[0]
	if assistantContent.Role != "model" {
		t.Errorf("expected role 'model', got %s", assistantContent.Role)
	}

	hasThought := false
	for _, part := range assistantContent.Parts {
		if part.Thought != nil && *part.Thought && part.Text == thinking {
			hasThought = true
		}
	}
	if !hasThought {
		t.Error("expected thought part with reasoning content")
	}
}

func TestGeminiModalityToTokenDetails(t *testing.T) {
	tests := []struct {
		modality string
		count    int
		field    string
		expected int
	}{
		{"TEXT", 100, "TextTokens", 100},
		{"IMAGE", 50, "ImageTokens", 50},
		{"AUDIO", 30, "AudioTokens", 30},
		{"UNKNOWN", 10, "", 0},
	}
	for _, tt := range tests {
		td := &common.TokenDetails{}
		geminiModalityToTokenDetails(dto.GeminiModalityTokenCount{Modality: tt.modality, TokenCount: tt.count}, td)
		var actual int
		switch tt.field {
		case "TextTokens":
			actual = td.TextTokens
		case "ImageTokens":
			actual = td.ImageTokens
		case "AudioTokens":
			actual = td.AudioTokens
		}
		if actual != tt.expected {
			t.Errorf("modality %q: got %d, want %d", tt.modality, actual, tt.expected)
		}
	}
}

func TestO2gParseStopSequences(t *testing.T) {
	tests := []struct {
		input    any
		expected []string
	}{
		{nil, nil},
		{"", nil},
		{"stop", []string{"stop"}},
		{[]string{"a", "b"}, []string{"a", "b"}},
		{[]any{"x", "y"}, []string{"x", "y"}},
		{42, nil},
	}
	for _, tt := range tests {
		result := o2gParseStopSequences(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("o2gParseStopSequences(%v) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i, v := range result {
			if v != tt.expected[i] {
				t.Errorf("o2gParseStopSequences(%v)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
			}
		}
	}
}

func TestGeminiDTOJsonMarshal(t *testing.T) {
	// Verify new DTO fields serialize correctly
	t.Run("GeminiPart with FileData", func(t *testing.T) {
		part := dto.GeminiPart{
			FileData: &dto.GeminiFileData{
				MimeType: "video/mp4",
				FileURI:  "gs://bucket/file.mp4",
			},
		}
		data, err := json.Marshal(part)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var parsed dto.GeminiPart
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if parsed.FileData == nil || parsed.FileData.FileURI != "gs://bucket/file.mp4" {
			t.Errorf("FileData not preserved: %v", parsed.FileData)
		}
	})

	t.Run("GeminiFunctionCall with ID", func(t *testing.T) {
		fc := dto.GeminiFunctionCall{
			ID:           "call_123",
			FunctionName: "get_weather",
			Arguments:    map[string]any{"city": "NYC"},
		}
		data, err := json.Marshal(fc)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var parsed dto.GeminiFunctionCall
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if parsed.ID != "call_123" || parsed.FunctionName != "get_weather" {
			t.Errorf("FunctionCall fields not preserved: %+v", parsed)
		}
	})

	t.Run("GeminiGenerationConfig with new fields", func(t *testing.T) {
		gc := dto.GeminiGenerationConfig{
			PresencePenalty:    float64Ptr(0.5),
			FrequencyPenalty:   float64Ptr(0.3),
			ResponseLogprobs:   boolPtr(true),
			Logprobs:           intPtr(10),
			ResponseModalities: []string{"TEXT", "IMAGE"},
			MediaResolution:    "MEDIA_RESOLUTION_HIGH",
		}
		data, err := json.Marshal(gc)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var parsed dto.GeminiGenerationConfig
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if parsed.PresencePenalty == nil || *parsed.PresencePenalty != 0.5 {
			t.Errorf("PresencePenalty not preserved")
		}
		if len(parsed.ResponseModalities) != 2 || parsed.ResponseModalities[0] != "TEXT" {
			t.Errorf("ResponseModalities not preserved: %v", parsed.ResponseModalities)
		}
	})

	t.Run("GeminiUsageMetadata with modality details", func(t *testing.T) {
		um := dto.GeminiUsageMetadata{
			PromptTokenCount:     100,
			CandidatesTokenCount: 200,
			TotalTokenCount:      300,
			PromptTokensDetails: []dto.GeminiModalityTokenCount{
				{Modality: "TEXT", TokenCount: 80},
				{Modality: "IMAGE", TokenCount: 20},
			},
		}
		data, err := json.Marshal(um)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var parsed dto.GeminiUsageMetadata
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if len(parsed.PromptTokensDetails) != 2 {
			t.Errorf("PromptTokensDetails not preserved: %v", parsed.PromptTokensDetails)
		}
	})
}

func float64Ptr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool          { return &v }
func intPtr(v int) *int             { return &v }
