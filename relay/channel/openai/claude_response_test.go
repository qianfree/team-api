package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

// newClaudeInboundTestInfo 构造 Claude 入站转换测试用的 RelayInfo。
func newClaudeInboundTestInfo() *common.RelayInfo {
	return &common.RelayInfo{
		RequestID:       "req-claude-inbound",
		OriginModelName: "gpt-4o",
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta:     &common.ChannelMeta{},
	}
}

// TestOpenAIToClaudeResponse_UsageNilDetails 上游不返回 prompt_tokens_details（多数 OpenAI 兼容上游）
// 时不得空指针 panic，且无缓存可扣减。
func TestOpenAIToClaudeResponse_UsageNilDetails(t *testing.T) {
	info := newClaudeInboundTestInfo()
	openaiResp := &dto.ChatCompletionResponse{
		ID:      "chatcmpl-1",
		Model:   "gpt-4o",
		Choices: []dto.Choice{{Index: 0, Message: dto.Message{Role: "assistant", Content: "hi"}, FinishReason: "stop"}},
		Usage:   dto.UsageWithDetails{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120},
	}

	resp := openAIToClaudeResponse(openaiResp, info) // PromptTokensDetails 为 nil，此处原会 panic

	if resp.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100 (no cache to deduct)", resp.Usage.InputTokens)
	}
	if resp.Usage.CacheReadInputTokens != 0 {
		t.Errorf("CacheReadInputTokens = %d, want 0", resp.Usage.CacheReadInputTokens)
	}
	if resp.Usage.OutputTokens != 20 {
		t.Errorf("OutputTokens = %d, want 20", resp.Usage.OutputTokens)
	}
}

// TestOpenAIToClaudeResponse_UsageWithCache 上游返回缓存明细时，usage 需按 Claude 语义映射：
// OpenAI 的 prompt_tokens 含 cached（子集），Claude 的 input_tokens 不含 cache_read，
// 直接透传会导致客户端按本协议语义计账时缓存部分重复计入。
func TestOpenAIToClaudeResponse_UsageWithCache(t *testing.T) {
	info := newClaudeInboundTestInfo()
	openaiResp := &dto.ChatCompletionResponse{
		ID:      "chatcmpl-2",
		Model:   "gpt-4o",
		Choices: []dto.Choice{{Index: 0, Message: dto.Message{Role: "assistant", Content: "hi"}, FinishReason: "stop"}},
		Usage: dto.UsageWithDetails{
			PromptTokens:        100,
			CompletionTokens:    20,
			TotalTokens:         120,
			PromptTokensDetails: &dto.TokenDetails{CachedTokens: 30},
		},
	}

	resp := openAIToClaudeResponse(openaiResp, info)

	if resp.Usage.InputTokens != 70 {
		t.Errorf("InputTokens = %d, want 70 (prompt 100 - cached 30)", resp.Usage.InputTokens)
	}
	if resp.Usage.CacheReadInputTokens != 30 {
		t.Errorf("CacheReadInputTokens = %d, want 30", resp.Usage.CacheReadInputTokens)
	}
}

// TestHandleClaudeInboundStream_CacheUsage 流式路径缓存用量：
//  1. 计费 usage 携带缓存明细并置 CacheIncludedInPrompt（计费扣减缓存部分，避免按 input 全价计费）；
//  2. 最终 message_delta 按 Claude 语义补报扣减后的 input_tokens 与 cache_read_input_tokens
//     （OpenAI 的 usage 在流尾才返回，message_start 无法提供真实值）。
func TestHandleClaudeInboundStream_CacheUsage(t *testing.T) {
	sse := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[],"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120,"prompt_tokens_details":{"cached_tokens":30},"completion_tokens_details":{"reasoning_tokens":5}}}

data: [DONE]

`
	upstream := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(sse)),
		Header:     http.Header{},
	}

	info := newClaudeInboundTestInfo()
	rec := httptest.NewRecorder()

	usage, err := handleClaudeInboundStream(context.Background(), upstream, info, rec)
	if err != nil {
		t.Fatalf("handleClaudeInboundStream error: %v", err)
	}

	// 计费侧：OpenAI 原始口径（prompt 含 cached）+ 明细 + 扣减标记
	if usage.PromptTokens != 100 || usage.CompletionTokens != 20 || usage.TotalTokens != 120 {
		t.Errorf("billing usage = %+v, want prompt=100 completion=20 total=120", usage)
	}
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 30 {
		t.Errorf("cache details = %+v, want cached=30", usage.PromptTokensDetails)
	}
	if !usage.CacheIncludedInPrompt {
		t.Error("CacheIncludedInPrompt = false, want true (OpenAI prompt 含 cached)")
	}

	// 客户端侧：message_delta 补报 Claude 语义 usage
	body := rec.Body.String()
	if !strings.Contains(body, `"input_tokens":70`) {
		t.Errorf("message_delta missing deducted input_tokens=70, got: %s", body)
	}
	if !strings.Contains(body, `"cache_read_input_tokens":30`) {
		t.Errorf("message_delta missing cache_read_input_tokens=30, got: %s", body)
	}
	if !strings.Contains(body, `"output_tokens":20`) {
		t.Errorf("message_delta missing output_tokens=20, got: %s", body)
	}
}
