package shared

import (
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

func TestMapOpenAIToolsToClaudeTools(t *testing.T) {
	parameters := json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`)

	tools := []dto.Tool{
		{
			Type: "function",
			Function: dto.FunctionDef{
				Name:        "get_weather",
				Description: "Get current weather",
				Parameters:  parameters,
			},
		},
	}

	result := MapOpenAIToolsToClaudeTools(tools)

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}

	if result[0].Name != "get_weather" {
		t.Errorf("Expected name 'get_weather', got %q", result[0].Name)
	}

	if result[0].Description != "Get current weather" {
		t.Errorf("Expected description 'Get current weather', got %q", result[0].Description)
	}
}

func TestMapClaudeToolsToOpenAITools(t *testing.T) {
	inputSchema := json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`)

	tools := []dto.ClaudeTool{
		{
			Name:        "get_weather",
			Description: "Get current weather",
			InputSchema: inputSchema,
		},
	}

	result := MapClaudeToolsToOpenAITools(tools)

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}

	if result[0].Type != "function" {
		t.Errorf("Expected type 'function', got %q", result[0].Type)
	}

	if result[0].Function.Name != "get_weather" {
		t.Errorf("Expected name 'get_weather', got %q", result[0].Function.Name)
	}
}

func TestMapClaudeToolCallsToOpenAI(t *testing.T) {
	input := map[string]any{
		"location": "San Francisco",
	}

	blocks := []dto.ClaudeContentBlock{
		{
			Type: "text",
			Text: strPtr("Let me check the weather"),
		},
		{
			Type:  "tool_use",
			ID:    "call_123",
			Name:  "get_weather",
			Input: input,
		},
	}

	result := MapClaudeToolCallsToOpenAI(blocks)

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result))
	}

	if result[0].ID != "call_123" {
		t.Errorf("Expected ID 'call_123', got %q", result[0].ID)
	}

	if result[0].Type != "function" {
		t.Errorf("Expected type 'function', got %q", result[0].Type)
	}

	if result[0].Function.Name != "get_weather" {
		t.Errorf("Expected name 'get_weather', got %q", result[0].Function.Name)
	}

	// 校验 arguments 为合法 JSON
	var args map[string]any
	if err := json.Unmarshal([]byte(result[0].Function.Arguments), &args); err != nil {
		t.Errorf("Arguments are not valid JSON: %v", err)
	}

	if args["location"] != "San Francisco" {
		t.Errorf("Expected location 'San Francisco', got %v", args["location"])
	}
}

func TestMapOpenAIToolCallsToClaude(t *testing.T) {
	toolCalls := []dto.ToolCall{
		{
			ID:   "call_123",
			Type: "function",
			Function: dto.FunctionCall{
				Name:      "get_weather",
				Arguments: `{"location":"San Francisco"}`,
			},
		},
	}

	result := MapOpenAIToolCallsToClaude(toolCalls)

	if len(result) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(result))
	}

	if result[0].Type != "tool_use" {
		t.Errorf("Expected type 'tool_use', got %q", result[0].Type)
	}

	if result[0].ID != "call_123" {
		t.Errorf("Expected ID 'call_123', got %q", result[0].ID)
	}

	if result[0].Name != "get_weather" {
		t.Errorf("Expected name 'get_weather', got %q", result[0].Name)
	}

	// 校验 input 已正确解析
	input, ok := result[0].Input.(map[string]any)
	if !ok {
		t.Fatalf("Expected input to be map[string]any, got %T", result[0].Input)
	}

	if input["location"] != "San Francisco" {
		t.Errorf("Expected location 'San Francisco', got %v", input["location"])
	}
}

// 无参函数（OpenAI 允许省略 parameters）必须补出空对象 schema：
// Claude 的 custom tool 要求 input_schema 必填，缺失会被上游以 400 拒绝。
func TestMapOpenAIToolsToClaudeTools_NoParameters(t *testing.T) {
	tools := []dto.Tool{
		{
			Type: "function",
			Function: dto.FunctionDef{
				Name:        "get_current_time",
				Description: "No arguments needed",
			},
		},
	}

	result := MapOpenAIToolsToClaudeTools(tools)

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}

	if result[0].InputSchema == nil {
		t.Fatal("InputSchema is nil, expected empty object schema")
	}

	// 序列化后必须带 input_schema 键（omitempty 不能把它抹掉）
	data, err := json.Marshal(result[0])
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	schema, ok := decoded["input_schema"].(map[string]any)
	if !ok {
		t.Fatalf("input_schema missing or wrong type in %s", data)
	}

	if schema["type"] != "object" {
		t.Errorf("input_schema.type = %v, want object", schema["type"])
	}

	if _, ok := schema["properties"].(map[string]any); !ok {
		t.Errorf("input_schema.properties missing in %s", data)
	}
}

// 非 function 类型的工具被过滤后返回空 slice，调用方据此判空。
func TestMapOpenAIToolsToClaudeTools_AllFiltered(t *testing.T) {
	tools := []dto.Tool{
		{Type: "custom", Function: dto.FunctionDef{Name: "freeform"}},
	}

	if result := MapOpenAIToolsToClaudeTools(tools); len(result) != 0 {
		t.Errorf("Expected 0 tools, got %d", len(result))
	}
}
