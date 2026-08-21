package oai_chat

// 回归测试：openai→claude 请求转换的内容/参数丢失类 bug（审查发现，修复后固化）。

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

// TestOpenAIToClaudeRequestConverter_ArrayContentFromJSON 回归：宿主 JSON 直解产出的
// content 为 []any（元素 map[string]any），此前 type switch 不识别该形态导致文本与
// 图片全部丢失（消息内容被兜底替换为单个空格）。
func TestOpenAIToClaudeRequestConverter_ArrayContentFromJSON(t *testing.T) {
	const raw = `{"model":"gpt-4","max_tokens":256,"messages":[{"role":"user","content":[` +
		`{"type":"text","text":"What is in this image?"},` +
		`{"type":"image_url","image_url":{"url":"data:image/png;base64,aGk="}}]}]}`
	var req dto.GeneralOpenAIRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	result, err := (&OpenAIToClaudeRequestConverter{}).ConvertRequest(
		context.Background(), &mockMeta{upstreamModel: "claude-3-5-sonnet-20241022"}, &req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	claudeReq := result.(*dto.ClaudeRequest)

	blocks, ok := claudeReq.Messages[0].Content.([]dto.ClaudeContentBlock)
	if !ok || len(blocks) != 2 {
		t.Fatalf("messages[0].content = %#v, want 2 blocks", claudeReq.Messages[0].Content)
	}
	if blocks[0].Type != "text" || blocks[0].Text == nil || *blocks[0].Text != "What is in this image?" {
		t.Errorf("text block = %#v, want text \"What is in this image?\"", blocks[0])
	}
	if blocks[1].Type != "image" || blocks[1].Source == nil {
		t.Fatalf("image block = %#v, want image with source", blocks[1])
	}
	if blocks[1].Source.MediaType != "image/png" || blocks[1].Source.Data != "aGk=" {
		t.Errorf("image source = %#v, want mediaType=image/png data=aGk=", blocks[1].Source)
	}
}

// TestOpenAIToClaudeRequestConverter_ReasoningEffort 回归：请求体 reasoning_effort →
// thinking 映射曾在 relaykit 迁移中丢失（legacy o2cConvertReasoningEffort 未收编）。
func TestOpenAIToClaudeRequestConverter_ReasoningEffort(t *testing.T) {
	maxTokens := 4096
	req := &dto.GeneralOpenAIRequest{
		Model:           "gpt-4",
		MaxTokens:       &maxTokens,
		ReasoningEffort: "high",
		Messages:        []dto.Message{{Role: "user", Content: "hi"}},
	}

	result, err := (&OpenAIToClaudeRequestConverter{}).ConvertRequest(
		context.Background(), &mockMeta{upstreamModel: "claude-3-7-sonnet-20250219"}, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	claudeReq := result.(*dto.ClaudeRequest)

	if claudeReq.Thinking == nil {
		t.Fatal("reasoning_effort=high 未映射为 thinking（迁移丢失回归）")
	}
	if claudeReq.Thinking.Type != "enabled" {
		t.Errorf("thinking.type = %q, want enabled", claudeReq.Thinking.Type)
	}
	if claudeReq.Thinking.BudgetTokens == nil || *claudeReq.Thinking.BudgetTokens != 32768 {
		t.Errorf("thinking.budget_tokens = %v, want 32768", claudeReq.Thinking.BudgetTokens)
	}
	// thinking 与 temperature 修改不兼容，须强制 1.0
	if claudeReq.Temperature == nil || *claudeReq.Temperature != 1.0 {
		t.Errorf("temperature = %v, want 1.0", claudeReq.Temperature)
	}
}

// TestOpenAIToClaudeRequestConverter_MaxCompletionTokens 回归：仅发送
// max_completion_tokens 的新式客户端（o 系列/gpt-5 系 SDK）此前被忽略后报错
// "max_tokens is required"。
func TestOpenAIToClaudeRequestConverter_MaxCompletionTokens(t *testing.T) {
	maxCompletion := 512
	req := &dto.GeneralOpenAIRequest{
		Model:               "gpt-4",
		MaxCompletionTokens: &maxCompletion,
		Messages:            []dto.Message{{Role: "user", Content: "hi"}},
	}

	result, err := (&OpenAIToClaudeRequestConverter{}).ConvertRequest(
		context.Background(), &mockMeta{upstreamModel: "claude-3-5-sonnet-20241022"}, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	claudeReq := result.(*dto.ClaudeRequest)
	if claudeReq.MaxTokens == nil || *claudeReq.MaxTokens != 512 {
		t.Errorf("max_tokens = %v, want 512（max_completion_tokens 应同等生效）", claudeReq.MaxTokens)
	}
}
