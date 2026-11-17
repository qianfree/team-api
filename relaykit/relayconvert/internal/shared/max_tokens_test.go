package shared

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

func TestIsOpenAIReasoningModel(t *testing.T) {
	cases := map[string]bool{
		"o1":                 true,
		"o1-mini":            true,
		"o3-mini-2025-01-31": true,
		"o4-mini":            true,
		"gpt-5":              true,
		"gpt-5.1":            true,
		"gpt-5-codex":        true,
		"GPT-5-Mini":         true,
		"openai/gpt-5":       true, // 聚合器 vendor/model 命名
		"codex-mini-latest":  true,
		"gpt-4o":             false,
		"gpt-4.1-mini":       false,
		"gpt-oss-120b":       false, // 开源推理模型由 vLLM 类服务承载，仍走 max_tokens
		"o1xyz":              false, // 前缀后必须是结尾或版本分隔符
		"claude-sonnet-4":    false,
		"deepseek-reasoner":  false,
		"":                   false,
	}
	for model, want := range cases {
		if got := IsOpenAIReasoningModel(model); got != want {
			t.Errorf("IsOpenAIReasoningModel(%q) = %v, want %v", model, got, want)
		}
	}
}

func TestSetChatMaxTokens(t *testing.T) {
	// 推理模型：只写 max_completion_tokens，且清掉可能残留的 max_tokens（OpenAI 两者同在也 400）
	stale := 1
	req := &dto.GeneralOpenAIRequest{Model: "gpt-5", MaxTokens: &stale}
	SetChatMaxTokens(req, 4096)
	if req.MaxTokens != nil {
		t.Errorf("reasoning model: MaxTokens = %d, want nil", *req.MaxTokens)
	}
	if req.MaxCompletionTokens == nil || *req.MaxCompletionTokens != 4096 {
		t.Errorf("reasoning model: MaxCompletionTokens = %v, want 4096", req.MaxCompletionTokens)
	}

	// 普通模型：写 max_tokens
	req = &dto.GeneralOpenAIRequest{Model: "gpt-4o"}
	SetChatMaxTokens(req, 2000)
	if req.MaxTokens == nil || *req.MaxTokens != 2000 {
		t.Errorf("plain model: MaxTokens = %v, want 2000", req.MaxTokens)
	}
	if req.MaxCompletionTokens != nil {
		t.Errorf("plain model: MaxCompletionTokens = %d, want nil", *req.MaxCompletionTokens)
	}

	// 非正值不写任何字段
	req = &dto.GeneralOpenAIRequest{Model: "gpt-4o"}
	SetChatMaxTokens(req, 0)
	if req.MaxTokens != nil || req.MaxCompletionTokens != nil {
		t.Error("non-positive maxTokens should leave both fields nil")
	}
	SetChatMaxTokens(nil, 10) // nil 请求不得 panic
}
