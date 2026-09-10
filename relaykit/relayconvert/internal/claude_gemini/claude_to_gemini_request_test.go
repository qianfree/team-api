package claude_gemini

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/oai_chat"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/oai_gemini"
)

// c2gUintPtr 返回 uint 的指针
func c2gUintPtr(v uint) *uint { return &v }

// runClaudeToGeminiRequest 跑直连请求转换并断言产物类型
func runClaudeToGeminiRequest(t *testing.T, req *dto.ClaudeRequest) *dto.GeminiChatRequest {
	t.Helper()
	conv := &ClaudeToGeminiRequestConverter{}
	got, err := conv.ConvertRequest(context.Background(), claudeInboundMeta(false), req)
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}
	geminiReq, ok := got.(*dto.GeminiChatRequest)
	if !ok {
		t.Fatalf("产物类型应为 *dto.GeminiChatRequest, got %T", got)
	}
	return geminiReq
}

// TestClaudeToGeminiRequest_BasicConversation 校验 system 块结构、角色映射与生成参数。
func TestClaudeToGeminiRequest_BasicConversation(t *testing.T) {
	req := &dto.ClaudeRequest{
		Model:     "claude-sonnet-4",
		MaxTokens: c2gUintPtr(1024),
		System: []any{
			map[string]any{"type": "text", "text": "你是助手"},
			map[string]any{"type": "text", "text": "回答简洁"},
		},
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "你好"},
			{Role: "assistant", Content: []any{map[string]any{"type": "text", "text": "有什么可以帮你"}}},
			{Role: "user", Content: []any{map[string]any{"type": "text", "text": "继续"}}},
		},
	}
	got := runClaudeToGeminiRequest(t, req)

	if got.SystemInstruction == nil || len(got.SystemInstruction.Parts) != 2 {
		t.Fatalf("systemInstruction 应保留两个文本块（链式会压平为一个）, got %+v", got.SystemInstruction)
	}
	if got.SystemInstruction.Parts[1].Text != "回答简洁" {
		t.Errorf("system 第二块文本错误: %+v", got.SystemInstruction.Parts[1])
	}

	if len(got.Contents) != 3 {
		t.Fatalf("contents 数量应为 3, got %d", len(got.Contents))
	}
	roles := []string{got.Contents[0].Role, got.Contents[1].Role, got.Contents[2].Role}
	if roles[0] != "user" || roles[1] != "model" || roles[2] != "user" {
		t.Errorf("角色映射应为 user/model/user, got %v", roles)
	}
	if got.Contents[0].Parts[0].Text != "你好" {
		t.Errorf("首条 user 文本错误: %+v", got.Contents[0].Parts[0])
	}

	if got.GenerationConfig.MaxOutputTokens == nil || *got.GenerationConfig.MaxOutputTokens != 1024 {
		t.Errorf("maxOutputTokens 应为 1024, got %+v", got.GenerationConfig.MaxOutputTokens)
	}
}

// TestClaudeToGeminiRequest_MaxTokensFallback 源请求缺 max_tokens 时兜底 4096。
func TestClaudeToGeminiRequest_MaxTokensFallback(t *testing.T) {
	got := runClaudeToGeminiRequest(t, &dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
	})
	if got.GenerationConfig.MaxOutputTokens == nil || *got.GenerationConfig.MaxOutputTokens != claudeMaxTokensFallback {
		t.Errorf("缺省 max_tokens 应兜底为 %d, got %+v",
			claudeMaxTokensFallback, got.GenerationConfig.MaxOutputTokens)
	}
}

// TestClaudeToGeminiRequest_Thinking 校验 budget 原值直传与 thinking 签名搬运。
func TestClaudeToGeminiRequest_Thinking(t *testing.T) {
	budget := 5000
	req := &dto.ClaudeRequest{
		Thinking: &dto.ClaudeThinking{Type: "enabled", BudgetTokens: &budget},
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "问题"},
			{Role: "assistant", Content: []any{
				map[string]any{"type": "thinking", "thinking": "让我想想", "signature": "sig-abc"},
				map[string]any{"type": "text", "text": "答案是 42"},
			}},
		},
	}
	got := runClaudeToGeminiRequest(t, req)

	tc := got.GenerationConfig.ThinkingConfig
	if tc == nil {
		t.Fatal("thinkingConfig 未生成")
	}
	if tc.ThoughtBudget == nil || *tc.ThoughtBudget != 5000 {
		t.Errorf("thoughtBudget 应为精确值 5000（链式会放大为 8192）, got %+v", tc.ThoughtBudget)
	}
	if tc.ThinkingLevel != "MEDIUM" {
		t.Errorf("5000 应落在 MEDIUM 档, got %q", tc.ThinkingLevel)
	}
	if !tc.IncludeThoughts {
		t.Error("includeThoughts 应为 true")
	}

	parts := got.Contents[1].Parts
	if len(parts) != 2 {
		t.Fatalf("assistant parts 数量应为 2, got %d", len(parts))
	}
	if parts[0].Thought == nil || !*parts[0].Thought {
		t.Errorf("思考块 thought 标记缺失: %+v", parts[0])
	}
	if parts[0].ThoughtSignature != "sig-abc" {
		t.Errorf("thinking 签名应搬运到 thoughtSignature（链式会丢失）, got %q", parts[0].ThoughtSignature)
	}
}

// TestClaudeToGeminiRequest_ThinkingDisabled 非 enabled 时不附加 thinkingConfig。
func TestClaudeToGeminiRequest_ThinkingDisabled(t *testing.T) {
	got := runClaudeToGeminiRequest(t, &dto.ClaudeRequest{
		Thinking: &dto.ClaudeThinking{Type: "disabled"},
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
	})
	if got.GenerationConfig.ThinkingConfig != nil {
		t.Errorf("thinking.type=disabled 不应生成 thinkingConfig, got %+v", got.GenerationConfig.ThinkingConfig)
	}
}

// TestClaudeToGeminiRequest_ToolsAndToolResult 校验工具声明、tool_choice 与工具结果回传。
func TestClaudeToGeminiRequest_ToolsAndToolResult(t *testing.T) {
	req := &dto.ClaudeRequest{
		Tools: []dto.ClaudeTool{
			{
				Name:        "get_weather",
				Description: "查天气",
				InputSchema: map[string]any{
					"type":                 "object",
					"properties":           map[string]any{"city": map[string]any{"type": "string"}},
					"required":             []any{"city"},
					"additionalProperties": false, // Gemini 不支持，应被过滤
					"$schema":              "https://json-schema.org/draft/2020-12/schema",
				},
			},
			{Name: "web_search", Type: "web_search_20250305"}, // 内置工具无 schema，应跳过
		},
		ToolChoice: map[string]any{"type": "tool", "name": "get_weather"},
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "北京天气"},
			{Role: "assistant", Content: []any{map[string]any{
				"type": "tool_use", "id": "toolu_01", "name": "get_weather",
				"input": map[string]any{"city": "北京"},
			}}},
			{Role: "user", Content: []any{map[string]any{
				"type": "tool_result", "tool_use_id": "toolu_01",
				"content": []any{map[string]any{"type": "text", "text": "晴 25 度"}},
			}}},
		},
	}
	got := runClaudeToGeminiRequest(t, req)

	var tools struct {
		FunctionDeclarations []dto.GeminiFunctionDeclaration `json:"functionDeclarations"`
	}
	if err := json.Unmarshal(got.Tools, &tools); err != nil {
		t.Fatalf("tools 反序列化失败: %v", err)
	}
	if len(tools.FunctionDeclarations) != 1 {
		t.Fatalf("应只保留 1 个函数声明（内置工具跳过）, got %d", len(tools.FunctionDeclarations))
	}
	params, _ := tools.FunctionDeclarations[0].Parameters.(map[string]any)
	if _, ok := params["additionalProperties"]; ok {
		t.Error("additionalProperties 应被过滤（Gemini 不支持）")
	}
	if _, ok := params["$schema"]; ok {
		t.Error("$schema 应被过滤（Gemini 不支持）")
	}
	if _, ok := params["properties"]; !ok {
		t.Error("properties 应保留")
	}

	toolConfig, _ := got.ToolConfig.(map[string]any)
	fcc, _ := toolConfig["functionCallingConfig"].(map[string]any)
	if fcc["mode"] != "ANY" {
		t.Errorf("tool_choice type=tool 应映射为 ANY, got %+v", fcc)
	}
	if names, _ := fcc["allowedFunctionNames"].([]string); len(names) != 1 || names[0] != "get_weather" {
		t.Errorf("allowedFunctionNames 应为 [get_weather], got %+v", fcc["allowedFunctionNames"])
	}

	call := got.Contents[1].Parts[0].FunctionCall
	if call == nil {
		t.Fatal("assistant tool_use 应转换为 functionCall")
	}
	if call.ID != "toolu_01" {
		t.Errorf("functionCall.id 应搬运 Claude 的 tool_use.id（链式会丢失）, got %q", call.ID)
	}
	if call.FunctionName != "get_weather" {
		t.Errorf("functionCall.name 错误: %q", call.FunctionName)
	}

	resp := got.Contents[2].Parts[0].FunctionResponse
	if resp == nil {
		t.Fatal("user tool_result 应转换为 functionResponse")
	}
	if resp.Name != "get_weather" {
		t.Errorf("functionResponse 应按 tool_use_id 关联出函数名, got %q", resp.Name)
	}
	if m, ok := resp.Response.(map[string]any); !ok || m["result"] != "晴 25 度" {
		t.Errorf("functionResponse.response 应为对象 {\"result\": ...}, got %#v", resp.Response)
	}
}

// TestClaudeToGeminiRequest_ToolResultJSONString JSON 字符串形态的工具结果直接解析为对象。
func TestClaudeToGeminiRequest_ToolResultJSONString(t *testing.T) {
	req := &dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{
			{Role: "assistant", Content: []any{map[string]any{
				"type": "tool_use", "id": "toolu_02", "name": "calc", "input": map[string]any{},
			}}},
			{Role: "user", Content: []any{map[string]any{
				"type": "tool_result", "tool_use_id": "toolu_02", "content": `{"v":25}`,
			}}},
		},
	}
	got := runClaudeToGeminiRequest(t, req)

	resp := got.Contents[1].Parts[0].FunctionResponse
	if resp == nil {
		t.Fatal("functionResponse 未生成")
	}
	if m, ok := resp.Response.(map[string]any); !ok || m["v"] != float64(25) {
		t.Errorf("JSON 字符串结果应解析为对象, got %#v", resp.Response)
	}
}

// TestClaudeToGeminiRequest_TopKAndStopSequences top_k 保留，stop_sequences 按 Gemini 上限截断。
func TestClaudeToGeminiRequest_TopKAndStopSequences(t *testing.T) {
	topK := 40
	got := runClaudeToGeminiRequest(t, &dto.ClaudeRequest{
		TopK:          &topK,
		StopSequences: []string{"a", "b", "c", "d", "e", "f"},
		Messages:      []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
	})

	if got.GenerationConfig.TopK == nil || *got.GenerationConfig.TopK != 40 {
		t.Errorf("topK 应保留（链式无对应字段会丢失）, got %+v", got.GenerationConfig.TopK)
	}
	if len(got.GenerationConfig.StopSequences) != geminiMaxStopSequences {
		t.Errorf("stopSequences 应截断为 %d 个, got %d",
			geminiMaxStopSequences, len(got.GenerationConfig.StopSequences))
	}
}

// TestClaudeToGeminiRequest_MediaBlocks base64 走 inlineData，URL 走 fileData。
func TestClaudeToGeminiRequest_MediaBlocks(t *testing.T) {
	got := runClaudeToGeminiRequest(t, &dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{{Role: "user", Content: []any{
			map[string]any{"type": "text", "text": "看图"},
			map[string]any{"type": "image", "source": map[string]any{
				"type": "base64", "media_type": "image/png", "data": "AAAA",
			}},
			map[string]any{"type": "image", "source": map[string]any{
				"type": "url", "media_type": "image/jpeg", "url": "https://example.com/a.jpg",
			}},
		}}},
	})

	parts := got.Contents[0].Parts
	if len(parts) != 3 {
		t.Fatalf("parts 数量应为 3, got %d", len(parts))
	}
	if parts[1].InlineData == nil || parts[1].InlineData.MimeType != "image/png" || parts[1].InlineData.Data != "AAAA" {
		t.Errorf("base64 图片应转为 inlineData, got %+v", parts[1])
	}
	if parts[2].FileData == nil || parts[2].FileData.FileURI != "https://example.com/a.jpg" {
		t.Errorf("URL 图片应转为 fileData, got %+v", parts[2])
	}
}

// TestClaudeToGeminiRequest_TypedBlocks content 已解析为 []dto.ClaudeContentBlock 时同样可用。
func TestClaudeToGeminiRequest_TypedBlocks(t *testing.T) {
	text := "答案"
	thinking := "推理"
	got := runClaudeToGeminiRequest(t, &dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "问题"},
			{Role: "assistant", Content: []dto.ClaudeContentBlock{
				{Type: "thinking", Thinking: &thinking, Signature: "sig-typed"},
				{Type: "text", Text: &text},
			}},
		},
	})

	parts := got.Contents[1].Parts
	if len(parts) != 2 {
		t.Fatalf("parts 数量应为 2, got %d", len(parts))
	}
	if parts[0].ThoughtSignature != "sig-typed" {
		t.Errorf("类型化 thinking 块签名未搬运, got %q", parts[0].ThoughtSignature)
	}
	if parts[1].Text != "答案" {
		t.Errorf("类型化 text 块文本错误: %q", parts[1].Text)
	}
}

// TestClaudeToGeminiRequest_EmptyPartsSkipped 无可搬运内容的消息不产出空 parts 的 content。
func TestClaudeToGeminiRequest_EmptyPartsSkipped(t *testing.T) {
	got := runClaudeToGeminiRequest(t, &dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: []any{
				map[string]any{"type": "redacted_thinking", "data": "加密块"},
				map[string]any{"type": "tool_result", "tool_use_id": "unknown", "content": "x"},
			}},
		},
	})
	if len(got.Contents) != 1 {
		t.Errorf("无可搬运内容的消息应整条跳过, contents=%+v", got.Contents)
	}
}

// TestClaudeToGeminiRequest_RejectsNonClaudeRequest 入参类型不符时报错。
func TestClaudeToGeminiRequest_RejectsNonClaudeRequest(t *testing.T) {
	conv := &ClaudeToGeminiRequestConverter{}
	if _, err := conv.ConvertRequest(context.Background(), claudeInboundMeta(false), "not a request"); err == nil {
		t.Fatal("非 *dto.ClaudeRequest 入参应返回错误")
	}
}

// TestClaudeToGeminiRequest_ConverterMetadata 转换器 ID / 方向 / 质量声明。
func TestClaudeToGeminiRequest_ConverterMetadata(t *testing.T) {
	conv := &ClaudeToGeminiRequestConverter{}
	if conv.ID() != "claude_messages_to_gemini_generate_content" {
		t.Errorf("ID 错误: %q", conv.ID())
	}
	if string(conv.From()) != "claude" || string(conv.To()) != "gemini" {
		t.Errorf("方向错误: %s → %s", conv.From(), conv.To())
	}
}

// TestClaudeToGeminiRequest_PreservesWhatChainLoses 直连相对「Claude→OpenAI→Gemini」
// 两跳链的保真度增益：精确 thinking budget、thinking 签名、top_k、tool_use.id 四项
// 均可经直连保留，而链式必然丢失。本用例即直连存在的理由。
func TestClaudeToGeminiRequest_PreservesWhatChainLoses(t *testing.T) {
	ctx := context.Background()
	meta := claudeInboundMeta(false)
	budget := 5000
	topK := 40
	req := &dto.ClaudeRequest{
		MaxTokens: c2gUintPtr(2048),
		TopK:      &topK,
		Thinking:  &dto.ClaudeThinking{Type: "enabled", BudgetTokens: &budget},
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "问题"},
			{Role: "assistant", Content: []any{
				map[string]any{"type": "thinking", "thinking": "推理", "signature": "sig-1"},
				map[string]any{"type": "tool_use", "id": "toolu_1", "name": "f", "input": map[string]any{}},
			}},
		},
	}

	conv := &ClaudeToGeminiRequestConverter{}
	directAny, err := conv.ConvertRequest(ctx, meta, req)
	if err != nil {
		t.Fatalf("直连转换失败: %v", err)
	}
	direct := directAny.(*dto.GeminiChatRequest)

	// 两跳链：Claude→OpenAI→Gemini
	mid, err := (&oai_chat.ClaudeToOpenAIRequestConverter{}).ConvertRequest(ctx, meta, req)
	if err != nil {
		t.Fatalf("第一跳转换失败: %v", err)
	}
	chainedAny, err := (&oai_gemini.OpenAIToGeminiRequestConverter{}).ConvertRequest(ctx, meta, mid)
	if err != nil {
		t.Fatalf("第二跳转换失败: %v", err)
	}
	chained := chainedAny.(*dto.GeminiChatRequest)

	// thinking budget：直连 5000，链式经 reasoning_effort 档位展开为 8192
	if b := direct.GenerationConfig.ThinkingConfig.ThoughtBudget; b == nil || *b != 5000 {
		t.Errorf("直连应保留精确 budget 5000, got %+v", b)
	}
	if b := chained.GenerationConfig.ThinkingConfig.ThoughtBudget; b == nil || *b != 8192 {
		t.Errorf("链式应放大为 8192（本用例前提）, got %+v", b)
	}

	// top_k：链式无对应字段
	if direct.GenerationConfig.TopK == nil {
		t.Error("直连应保留 topK")
	}
	if chained.GenerationConfig.TopK != nil {
		t.Errorf("链式应丢失 topK（本用例前提）, got %+v", chained.GenerationConfig.TopK)
	}

	// thinking 签名与 tool_use.id：链式经中枢格式丢弃
	if direct.Contents[1].Parts[0].ThoughtSignature != "sig-1" {
		t.Errorf("直连应保留 thinking 签名, got %q", direct.Contents[1].Parts[0].ThoughtSignature)
	}
	if chained.Contents[1].Parts[0].ThoughtSignature != "" {
		t.Errorf("链式应丢失 thinking 签名（本用例前提）, got %q", chained.Contents[1].Parts[0].ThoughtSignature)
	}
	if id := direct.Contents[1].Parts[1].FunctionCall.ID; id != "toolu_1" {
		t.Errorf("直连应保留 functionCall.id, got %q", id)
	}
}
