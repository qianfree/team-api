package oai_gemini

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

func TestGeminiToOpenAIRequestConverter_Metadata(t *testing.T) {
	converter := &GeminiToOpenAIRequestConverter{}

	if converter.ID() != relayconvert.ConverterGeminiContentToOpenAIChat {
		t.Errorf("ID() = %q, want %q", converter.ID(), relayconvert.ConverterGeminiContentToOpenAIChat)
	}

	if converter.From() != types.RelayFormatGemini {
		t.Errorf("From() = %q, want %q", converter.From(), types.RelayFormatGemini)
	}

	if converter.To() != types.RelayFormatOpenAI {
		t.Errorf("To() = %q, want %q", converter.To(), types.RelayFormatOpenAI)
	}

	if converter.Quality() != relayconvert.RequestConverterQualityGood {
		t.Errorf("Quality() = %q, want %q", converter.Quality(), relayconvert.RequestConverterQualityGood)
	}
}

func TestGeminiToOpenAIRequestConverter_BasicConversion(t *testing.T) {
	converter := &GeminiToOpenAIRequestConverter{}
	ctx := context.Background()
	info := &convmeta.Values{UpstreamModelName: "gpt-4o", ChannelMetaAttached: true}

	temp := 0.7
	topP := 0.9
	topK := 40.0
	maxTokens := uint(1024)
	seed := int64(42)

	geminiReq := &dto.GeminiChatRequest{
		SystemInstruction: &dto.GeminiContent{
			Parts: []dto.GeminiPart{{Text: "You are helpful."}, {Text: "Be brief."}},
		},
		Contents: []dto.GeminiContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "Hello"}}},
		},
		GenerationConfig: &dto.GeminiGenerationConfig{
			Temperature:     &temp,
			TopP:            &topP,
			TopK:            &topK,
			MaxOutputTokens: &maxTokens,
			StopSequences:   []string{"END"},
			Seed:            &seed,
		},
	}

	result, err := converter.ConvertRequest(ctx, info, geminiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	openaiReq, ok := result.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("Expected *dto.GeneralOpenAIRequest, got %T", result)
	}

	if openaiReq.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", openaiReq.Model, "gpt-4o")
	}
	if openaiReq.Temperature == nil || *openaiReq.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", openaiReq.Temperature)
	}
	if openaiReq.TopP == nil || *openaiReq.TopP != 0.9 {
		t.Errorf("TopP = %v, want 0.9", openaiReq.TopP)
	}
	if openaiReq.TopK == nil || *openaiReq.TopK != 40 {
		t.Errorf("TopK = %v, want 40", openaiReq.TopK)
	}
	if openaiReq.MaxTokens == nil || *openaiReq.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %v, want 1024", openaiReq.MaxTokens)
	}
	if openaiReq.Seed == nil || *openaiReq.Seed != 42 {
		t.Errorf("Seed = %v, want 42", openaiReq.Seed)
	}
	stops, ok := openaiReq.Stop.([]string)
	if !ok || len(stops) != 1 || stops[0] != "END" {
		t.Errorf("Stop = %v, want [END]", openaiReq.Stop)
	}

	// system 指令多段用换行拼接 + user 消息
	if len(openaiReq.Messages) != 2 {
		t.Fatalf("Messages count = %d, want 2", len(openaiReq.Messages))
	}
	if openaiReq.Messages[0].Role != "system" {
		t.Errorf("First message role = %q, want system", openaiReq.Messages[0].Role)
	}
	if openaiReq.Messages[0].Content != "You are helpful.\nBe brief." {
		t.Errorf("System content = %q", openaiReq.Messages[0].Content)
	}
	if openaiReq.Messages[1].Role != "user" || openaiReq.Messages[1].Content != "Hello" {
		t.Errorf("User message = %+v", openaiReq.Messages[1])
	}
}

func TestGeminiToOpenAIRequestConverter_ThinkingConfig(t *testing.T) {
	converter := &GeminiToOpenAIRequestConverter{}
	ctx := context.Background()

	budgetOf := func(v int) *int { return &v }

	tests := []struct {
		name   string
		tc     *dto.GeminiThinkingConfig
		effort string
	}{
		{"无预算默认 medium", &dto.GeminiThinkingConfig{}, "medium"},
		{"低预算", &dto.GeminiThinkingConfig{ThoughtBudget: budgetOf(1000)}, "low"},
		{"低预算边界", &dto.GeminiThinkingConfig{ThoughtBudget: budgetOf(2048)}, "low"},
		{"中预算", &dto.GeminiThinkingConfig{ThoughtBudget: budgetOf(8192)}, "medium"},
		{"中预算边界", &dto.GeminiThinkingConfig{ThoughtBudget: budgetOf(16384)}, "medium"},
		{"高预算", &dto.GeminiThinkingConfig{ThoughtBudget: budgetOf(32768)}, "high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			geminiReq := &dto.GeminiChatRequest{
				Contents:         []dto.GeminiContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}}},
				GenerationConfig: &dto.GeminiGenerationConfig{ThinkingConfig: tt.tc},
			}
			result, err := converter.ConvertRequest(ctx, nil, geminiReq)
			if err != nil {
				t.Fatalf("ConvertRequest failed: %v", err)
			}
			openaiReq := result.(*dto.GeneralOpenAIRequest)
			if openaiReq.ReasoningEffort != tt.effort {
				t.Errorf("ReasoningEffort = %q, want %q", openaiReq.ReasoningEffort, tt.effort)
			}
		})
	}
}

func TestGeminiToOpenAIRequestConverter_ResponseFormat(t *testing.T) {
	converter := &GeminiToOpenAIRequestConverter{}
	ctx := context.Background()

	// 只有 mimeType → json_object
	geminiReq := &dto.GeminiChatRequest{
		Contents:         []dto.GeminiContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}}},
		GenerationConfig: &dto.GeminiGenerationConfig{ResponseMimeType: "application/json"},
	}
	result, err := converter.ConvertRequest(ctx, nil, geminiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}
	openaiReq := result.(*dto.GeneralOpenAIRequest)
	if openaiReq.ResponseFormat == nil || openaiReq.ResponseFormat.Type != "json_object" {
		t.Errorf("ResponseFormat = %+v, want json_object", openaiReq.ResponseFormat)
	}

	// mimeType + schema → json_schema
	schema := map[string]any{"type": "object"}
	geminiReq.GenerationConfig.ResponseSchema = schema
	result, err = converter.ConvertRequest(ctx, nil, geminiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}
	openaiReq = result.(*dto.GeneralOpenAIRequest)
	if openaiReq.ResponseFormat == nil || openaiReq.ResponseFormat.Type != "json_schema" {
		t.Errorf("ResponseFormat = %+v, want json_schema", openaiReq.ResponseFormat)
	}
}

func TestGeminiToOpenAIRequestConverter_ToolsAndToolConfig(t *testing.T) {
	converter := &GeminiToOpenAIRequestConverter{}
	ctx := context.Background()

	toolsJSON := json.RawMessage(`[{"functionDeclarations":[{"name":"get_weather","description":"查询天气","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}]}]`)

	geminiReq := &dto.GeminiChatRequest{
		Contents: []dto.GeminiContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "天气"}}}},
		Tools:    toolsJSON,
		ToolConfig: map[string]any{
			"functionCallingConfig": map[string]any{"mode": "ANY", "allowedFunctionNames": []any{"get_weather"}},
		},
	}

	result, err := converter.ConvertRequest(ctx, nil, geminiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}
	openaiReq := result.(*dto.GeneralOpenAIRequest)

	if len(openaiReq.Tools) != 1 {
		t.Fatalf("Tools count = %d, want 1", len(openaiReq.Tools))
	}
	if openaiReq.Tools[0].Type != "function" || openaiReq.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Tool = %+v", openaiReq.Tools[0])
	}
	if openaiReq.Tools[0].Function.Description != "查询天气" {
		t.Errorf("Tool description = %q", openaiReq.Tools[0].Function.Description)
	}

	// ANY + 单个函数名 → 强制指定函数
	tc, ok := openaiReq.ToolChoice.(map[string]any)
	if !ok {
		t.Fatalf("ToolChoice type = %T, want map", openaiReq.ToolChoice)
	}
	fn, _ := tc["function"].(map[string]any)
	if tc["type"] != "function" || fn["name"] != "get_weather" {
		t.Errorf("ToolChoice = %v", openaiReq.ToolChoice)
	}
}

func TestG2oConvertToolConfigModes(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want any
	}{
		{"NONE", map[string]any{"functionCallingConfig": map[string]any{"mode": "NONE"}}, "none"},
		{"AUTO", map[string]any{"functionCallingConfig": map[string]any{"mode": "AUTO"}}, "auto"},
		{"ANY 无函数名", map[string]any{"functionCallingConfig": map[string]any{"mode": "ANY"}}, "required"},
		{"未知 mode", map[string]any{"functionCallingConfig": map[string]any{"mode": "XXX"}}, "auto"},
		{"非 map 输入", "bad", nil},
		{"缺少 functionCallingConfig", map[string]any{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g2oConvertToolConfig(tt.in)
			if got != tt.want {
				t.Errorf("g2oConvertToolConfig(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestGeminiToOpenAIRequestConverter_FunctionCallHistory(t *testing.T) {
	converter := &GeminiToOpenAIRequestConverter{}
	ctx := context.Background()

	geminiReq := &dto.GeminiChatRequest{
		Contents: []dto.GeminiContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "查天气"}}},
			{Role: "model", Parts: []dto.GeminiPart{
				{FunctionCall: &dto.GeminiFunctionCall{FunctionName: "get_weather", Arguments: map[string]any{"city": "北京"}}},
			}},
			{Role: "user", Parts: []dto.GeminiPart{
				{FunctionResponse: &dto.GeminiFunctionResponse{Name: "get_weather", Response: map[string]any{"temp": 25}}},
			}},
		},
	}

	result, err := converter.ConvertRequest(ctx, nil, geminiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}
	openaiReq := result.(*dto.GeneralOpenAIRequest)

	// user + assistant(tool_calls) + tool
	if len(openaiReq.Messages) != 3 {
		t.Fatalf("Messages count = %d, want 3", len(openaiReq.Messages))
	}

	asst := openaiReq.Messages[1]
	if asst.Role != "assistant" || len(asst.ToolCalls) != 1 {
		t.Fatalf("Assistant message = %+v", asst)
	}
	if asst.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("ToolCall name = %q", asst.ToolCalls[0].Function.Name)
	}
	// 带 tool_calls 但无文本时 Content 需为空字符串而非 nil
	if asst.Content != "" {
		t.Errorf("Assistant content = %v, want empty string", asst.Content)
	}

	toolMsg := openaiReq.Messages[2]
	if toolMsg.Role != "tool" {
		t.Fatalf("Third message role = %q, want tool", toolMsg.Role)
	}
	// functionResponse 复用 functionCall 分配的 ID
	if toolMsg.ToolCallID != asst.ToolCalls[0].ID {
		t.Errorf("ToolCallID = %q, want %q", toolMsg.ToolCallID, asst.ToolCalls[0].ID)
	}
	if toolMsg.Content != `{"temp":25}` {
		t.Errorf("Tool content = %v", toolMsg.Content)
	}
}

func TestGeminiToOpenAIRequestConverter_InlineData(t *testing.T) {
	converter := &GeminiToOpenAIRequestConverter{}
	ctx := context.Background()

	geminiReq := &dto.GeminiChatRequest{
		Contents: []dto.GeminiContent{
			{Role: "user", Parts: []dto.GeminiPart{
				{Text: "这是什么图？"},
				{InlineData: &dto.GeminiInlineData{MimeType: "image/png", Data: "aGVsbG8="}},
			}},
		},
	}

	result, err := converter.ConvertRequest(ctx, nil, geminiReq)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}
	openaiReq := result.(*dto.GeneralOpenAIRequest)

	if len(openaiReq.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(openaiReq.Messages))
	}
	parts, ok := openaiReq.Messages[0].Content.([]dto.ContentPart)
	if !ok {
		t.Fatalf("Content type = %T, want []dto.ContentPart", openaiReq.Messages[0].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("Content parts = %d, want 2", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "这是什么图？" {
		t.Errorf("Text part = %+v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil {
		t.Fatalf("Image part = %+v", parts[1])
	}
	if parts[1].ImageURL.URL != "data:image/png;base64,aGVsbG8=" {
		t.Errorf("Image URL = %q", parts[1].ImageURL.URL)
	}
	if parts[1].ImageURL.Detail != "auto" {
		t.Errorf("Image detail = %q, want auto", parts[1].ImageURL.Detail)
	}
}

func TestG2oMapRoleMapping(t *testing.T) {
	if g2oMapRole("model") != "assistant" {
		t.Errorf("g2oMapRole(model) = %q, want assistant", g2oMapRole("model"))
	}
	if g2oMapRole("user") != "user" {
		t.Errorf("g2oMapRole(user) = %q, want user", g2oMapRole("user"))
	}
}

func TestGeminiToOpenAIRequestConverter_InvalidRequestType(t *testing.T) {
	converter := &GeminiToOpenAIRequestConverter{}
	ctx := context.Background()

	if _, err := converter.ConvertRequest(ctx, nil, "not a request"); err == nil {
		t.Error("Expected error for invalid request type, got nil")
	}
}
