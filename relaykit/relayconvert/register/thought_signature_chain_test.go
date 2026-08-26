package register

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chainTestMeta 签名链路测试共用 Meta 桩。
func chainTestMeta() *convmeta.Values {
	return &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "gemini-3-pro",
		UpstreamModelName:   "gemini-3-pro",
	}
}

// TestClaudeToGeminiChain_ThoughtSignature 请求方向签名往返：claude 客户端回传的
// thinking 块 signature 经 claude→openai→gemini 两跳链回挂到 functionCall part 的
// thoughtSignature（Gemini 3 函数调用轮次强校验点）。
func TestClaudeToGeminiChain_ThoughtSignature(t *testing.T) {
	spec, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterClaudeMessagesToGeminiContent)
	require.True(t, ok, "claude→gemini chain not registered")

	maxTokens := uint(1024)
	claudeReq := &dto.ClaudeRequest{
		Model:     "gemini-3-pro",
		MaxTokens: &maxTokens,
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "查天气"},
			{Role: "assistant", Content: []any{
				map[string]any{"type": "thinking", "thinking": "思考", "signature": "sig-rt"},
				map[string]any{"type": "tool_use", "id": "t1", "name": "get_weather", "input": map[string]any{"city": "北京"}},
			}},
			{Role: "user", Content: []any{
				map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": "晴"},
			}},
		},
	}

	result, err := relayconvert.ExecuteRequestConverter(context.Background(), spec, chainTestMeta(), claudeReq)
	require.NoError(t, err)
	geminiReq, ok := result.(*dto.GeminiChatRequest)
	require.True(t, ok, "chain result type = %T, want *dto.GeminiChatRequest", result)

	var fcSig string
	for _, content := range geminiReq.Contents {
		for _, p := range content.Parts {
			if p.FunctionCall != nil && p.FunctionCall.FunctionName == "get_weather" {
				fcSig = p.ThoughtSignature
			}
		}
	}
	assert.Equal(t, "sig-rt", fcSig, "thinking 块签名应回挂到 functionCall part 的 thoughtSignature")
}

// TestGeminiToClaudeStreamChain_ThoughtSignature 流式响应方向签名往返：Gemini SSE 的
// functionCall thoughtSignature 经 g2o+o2c 两跳流式组合（io.Pipe JSON 序列化中转）
// 转出为 Claude thinking 块的 signature_delta，且先于 tool_use 块开启。
func TestGeminiToClaudeStreamChain_ThoughtSignature(t *testing.T) {
	fn, _, ok := relayconvert.LookupStreamConverter(types.RelayFormatGemini, types.RelayFormatClaude)
	require.True(t, ok, "gemini→claude stream chain not registered")

	sse := `data: {"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"city":"北京"}},"thoughtSignature":"sig-stream"}]},"index":0,"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}

`
	var events []*dto.ClaudeStreamEvent
	err := fn(context.Background(), chainTestMeta(), strings.NewReader(sse), func(chunk any) error {
		if e, ok := chunk.(*dto.ClaudeStreamEvent); ok {
			events = append(events, e)
		}
		return nil
	})
	require.NoError(t, err)

	sigIdx, toolStartIdx := -1, -1
	for i, e := range events {
		if e.Type == "content_block_delta" && e.Data.Delta != nil && e.Data.Delta.Type == "signature_delta" {
			sigIdx = i
			assert.Equal(t, "sig-stream", e.Data.Delta.Signature, "签名值应经 io.Pipe JSON 序列化无损透传")
		}
		if e.Type == "content_block_start" && e.Data.ContentBlock != nil && e.Data.ContentBlock.Type == "tool_use" {
			toolStartIdx = i
		}
	}
	require.NotEqual(t, -1, sigIdx, "应发出 signature_delta 事件")
	require.NotEqual(t, -1, toolStartIdx, "应发出 tool_use 块")
	assert.Less(t, sigIdx, toolStartIdx, "signature_delta 应先于 tool_use 块开启（thinking 承载块在前）")
}
