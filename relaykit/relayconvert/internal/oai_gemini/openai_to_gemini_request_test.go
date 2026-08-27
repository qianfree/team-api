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

	// 检查 generation config
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

	// 检查 system instruction
	if geminiReq.SystemInstruction == nil {
		t.Fatal("SystemInstruction is nil")
	}
	if len(geminiReq.SystemInstruction.Parts) != 1 {
		t.Fatalf("SystemInstruction parts count = %d, want 1", len(geminiReq.SystemInstruction.Parts))
	}
	if geminiReq.SystemInstruction.Parts[0].Text != "You are a helpful assistant." {
		t.Errorf("SystemInstruction text = %q, want %q", geminiReq.SystemInstruction.Parts[0].Text, "You are a helpful assistant.")
	}

	// 检查 contents
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

	// 检查 safety settings
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

	// 检查 tools
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

	// 检查 contents 结构：user -> model（含 functionCall） -> user（含 functionResponse）
	if len(geminiReq.Contents) != 3 {
		t.Fatalf("Contents count = %d, want 3", len(geminiReq.Contents))
	}

	// 第一条：user 消息
	if geminiReq.Contents[0].Role != "user" {
		t.Errorf("First content role = %q, want %q", geminiReq.Contents[0].Role, "user")
	}

	// 第二条：带 tool call 的 assistant 消息
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

	// 第三条：带 function response 的 user 消息
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

// 回归：非 JSON 对象的 tool 结果（纯文本/标量/数组）必须包进 {"result": ...}，
// Gemini 上游按 map 解析 functionResponse.response，裸字符串会报
// "cannot unmarshal string into Go struct field ... of type map[string]interface {}"
func TestOpenAIToGeminiRequestConverter_ToolResultNonObjectWrapped(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	cases := []struct {
		name    string
		content string
		want    any
	}{
		{"纯文本", "exec_command failed: exit 1", map[string]any{"result": "exec_command failed: exit 1"}},
		{"JSON 数组", `[1,2]`, map[string]any{"result": []any{float64(1), float64(2)}}},
		{"JSON 标量", `42`, map[string]any{"result": float64(42)}},
		{"空字符串", "", map[string]any{"result": ""}},
		{"JSON 对象保持原样", `{"ok":true}`, map[string]any{"ok": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			openaiReq := &dto.GeneralOpenAIRequest{
				Messages: []dto.Message{
					{Role: "tool", Content: tc.content, ToolCallID: "call_1", Name: "exec_command"},
				},
			}
			result, err := converter.ConvertRequest(ctx, nil, openaiReq)
			if err != nil {
				t.Fatalf("ConvertRequest failed: %v", err)
			}
			geminiReq := result.(*dto.GeminiChatRequest)
			if len(geminiReq.Contents) == 0 || len(geminiReq.Contents[0].Parts) == 0 {
				t.Fatal("no contents/parts generated")
			}
			fr := geminiReq.Contents[0].Parts[0].FunctionResponse
			if fr == nil {
				t.Fatal("FunctionResponse is nil")
			}
			got, want := mustJSON(t, fr.Response), mustJSON(t, tc.want)
			if got != want {
				t.Errorf("Response = %s, want %s", got, want)
			}
		})
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
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

	// 第一个 part：文本
	if parts[0].Text != "What's in this image?" {
		t.Errorf("First part text = %q, want %q", parts[0].Text, "What's in this image?")
	}

	// 第二个 part：图片内联数据
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

// TestOpenAIToGeminiRequestConverter_ReasoningEffort reasoning_effort → thinkingConfig。
//
// ⚠️ thinkingBudget 与 thinkingLevel 互斥：同时下发上游返回 400
// "thinking_budget and thinking_level are not supported together"。按模型代次二选一——
// Gemini 2.5 系只发 thinkingBudget，Gemini 3+ 只发 thinkingLevel。
func TestOpenAIToGeminiRequestConverter_ReasoningEffort(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		effort     string
		wantBudget int    // 0 表示不应设置
		wantLevel  string // "" 表示不应设置
	}{
		{name: "2.5 low effort", model: "gemini-2.5-pro", effort: "low", wantBudget: 1024},
		{name: "2.5 medium effort", model: "gemini-2.5-pro", effort: "medium", wantBudget: 8192},
		{name: "2.5 high effort", model: "gemini-2.5-flash", effort: "high", wantBudget: 32768},
		{name: "2.5 unknown effort defaults to medium", model: "gemini-2.5-pro", effort: "unknown", wantBudget: 8192},
		{name: "2.5 xhigh folds to high budget", model: "gemini-2.5-pro", effort: "xhigh", wantBudget: 32768},
		{name: "3 pro uses level not budget", model: "gemini-3-pro-preview", effort: "medium", wantLevel: "MEDIUM"},
		{name: "3 pro low", model: "gemini-3-pro-preview", effort: "low", wantLevel: "LOW"},
		{name: "3 pro xhigh folds to HIGH", model: "gemini-3-pro-preview", effort: "xhigh", wantLevel: "HIGH"},
		{name: "unknown model falls back to budget", model: "some-proxy-model", effort: "medium", wantBudget: 8192},
	}

	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openaiReq := &dto.GeneralOpenAIRequest{
				Model:           tt.model,
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

			tc := geminiReq.GenerationConfig.ThinkingConfig
			if !tc.IncludeThoughts {
				t.Error("IncludeThoughts should be true")
			}
			if tt.wantBudget == 0 {
				if tc.ThinkingBudget != nil {
					t.Errorf("ThinkingBudget = %d, want unset（与 thinkingLevel 互斥）", *tc.ThinkingBudget)
				}
			} else if tc.ThinkingBudget == nil || *tc.ThinkingBudget != tt.wantBudget {
				t.Errorf("ThinkingBudget = %v, want %d", tc.ThinkingBudget, tt.wantBudget)
			}
			if tc.ThinkingLevel != tt.wantLevel {
				t.Errorf("ThinkingLevel = %q, want %q（与 thinkingBudget 互斥）", tc.ThinkingLevel, tt.wantLevel)
			}
		})
	}
}

// TestGeminiThinkingConfigJSONFieldName 锁定 REST 字段名：必须是 thinkingBudget，
// 不是 thoughtBudget——后者不是合法字段，会被上游以 400 "Unknown name" 拒绝。
func TestGeminiThinkingConfigJSONFieldName(t *testing.T) {
	budget := 4096
	b, err := json.Marshal(&dto.GeminiThinkingConfig{IncludeThoughts: true, ThinkingBudget: &budget})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"thinkingBudget":4096`) {
		t.Errorf("thinkingConfig = %s, want thinkingBudget 字段", got)
	}
	if strings.Contains(got, "thoughtBudget") {
		t.Errorf("thinkingConfig 含非法字段 thoughtBudget: %s", got)
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

	// 检查 schema 类型已转换为大写
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

			// 若适用，检查特定函数名
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

	// 检查 assistant 消息
	assistantContent := geminiReq.Contents[1]
	if assistantContent.Role != "model" {
		t.Errorf("Assistant role = %q, want %q", assistantContent.Role, "model")
	}

	// 应有 2 个 part：常规文本 + thought
	if len(assistantContent.Parts) != 2 {
		t.Fatalf("Assistant parts count = %d, want 2", len(assistantContent.Parts))
	}

	// 第一个 part：常规文本
	if assistantContent.Parts[0].Text != "The answer is 42" {
		t.Errorf("First part text = %q, want %q", assistantContent.Parts[0].Text, "The answer is 42")
	}

	// 第二个 part：thought
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

// TestOpenAIToGeminiRequestConverter_WebSearchOptions web_search_options（responses 入站
// 的 web_search 托管工具经 r2c 提取）映射为 googleSearch grounding tool 条目；
// 与 functionDeclarations 条目并存。
func TestOpenAIToGeminiRequestConverter_WebSearchOptions(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	openaiReq := &dto.GeneralOpenAIRequest{
		WebSearchOptions: json.RawMessage(`{"search_context_size":"high"}`),
		Messages:         []dto.Message{{Role: "user", Content: "search news"}},
		Tools: []dto.Tool{{
			Type: "function",
			Function: dto.FunctionDef{
				Name:       "f1",
				Parameters: map[string]any{"type": "object"},
			},
		}},
	}

	result, err := converter.ConvertRequest(ctx, nil, openaiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}
	geminiReq := result.(*dto.GeminiChatRequest)

	if geminiReq.Tools == nil {
		t.Fatal("Tools is nil")
	}
	var tools []geminiTool
	if err := json.Unmarshal(geminiReq.Tools, &tools); err != nil {
		t.Fatalf("Failed to unmarshal tools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("Tools count = %d, want 2 (functionDeclarations + googleSearch)", len(tools))
	}
	if len(tools[0].FunctionDeclarations) != 1 || tools[0].FunctionDeclarations[0].Name != "f1" {
		t.Errorf("tools[0] = %+v, want f1 declarations", tools[0])
	}
	if tools[1].GoogleSearch == nil {
		t.Errorf("tools[1] = %+v, want googleSearch entry", tools[1])
	}
}

// TestOpenAIToGeminiRequestConverter_WebSearchOptionsOnly 无 function 工具时
// googleSearch grounding 独立生效。
func TestOpenAIToGeminiRequestConverter_WebSearchOptionsOnly(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	openaiReq := &dto.GeneralOpenAIRequest{
		WebSearchOptions: json.RawMessage(`{}`),
		Messages:         []dto.Message{{Role: "user", Content: "search news"}},
	}

	result, err := converter.ConvertRequest(ctx, nil, openaiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}
	geminiReq := result.(*dto.GeminiChatRequest)

	var tools []geminiTool
	if err := json.Unmarshal(geminiReq.Tools, &tools); err != nil {
		t.Fatalf("Failed to unmarshal tools: %v", err)
	}
	if len(tools) != 1 || tools[0].GoogleSearch == nil {
		t.Fatalf("tools = %+v, want single googleSearch entry", tools)
	}
}
