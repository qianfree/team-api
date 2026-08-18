package register

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResponsesToClaudeStreamChain responses 上游 → claude 客户端的流式组合链 e2e
//（B 流式 + openai→claude 流式，经 io.Pipe 串联）。
func TestResponsesToClaudeStreamChain(t *testing.T) {
	sse := "data: " + `{"type":"response.created","response":{"id":"resp_1","model":"gpt-4o","created_at":1730000000}}` + "\n\n" +
		"data: " + `{"type":"response.output_text.delta","delta":"你好"}` + "\n\n" +
		"data: " + `{"type":"response.completed","response":{"model":"gpt-4o","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}` + "\n\n"

	fn, id, ok := relayconvert.LookupStreamConverter(types.RelayFormatOpenAIResponses, types.RelayFormatClaude)
	require.True(t, ok, "responses→claude 流式组合链应已注册")
	assert.NotEmpty(t, id)

	var events []*dto.ClaudeStreamEvent
	err := fn(context.Background(), nil, strings.NewReader(sse), func(chunk any) error {
		if e, ok := chunk.(*dto.ClaudeStreamEvent); ok {
			events = append(events, e)
		}
		return nil
	})
	require.NoError(t, err)

	var types []string
	for _, e := range events {
		types = append(types, e.Type)
	}
	// 期望：message_start → text 块 start/delta → stop → message_delta → message_stop
	assert.Contains(t, types, "message_start")
	assert.Contains(t, types, "content_block_delta")
	assert.Contains(t, types, "message_stop")
	// usage 经 message_delta 透出（Claude 扣减口径：cached=0 时 input=10）
	for _, e := range events {
		if e.Type == "message_delta" && e.Data.Usage != nil {
			assert.Equal(t, 10, e.Data.Usage.InputTokens)
			assert.Equal(t, 5, e.Data.Usage.OutputTokens)
		}
	}
}

// TestResponsesToGeminiStreamChain responses 上游 → gemini 客户端的流式组合链 e2e。
func TestResponsesToGeminiStreamChain(t *testing.T) {
	sse := "data: " + `{"type":"response.output_text.delta","delta":"hi"}` + "\n\n" +
		"data: " + `{"type":"response.completed","response":{"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}` + "\n\n"

	fn, _, ok := relayconvert.LookupStreamConverter(types.RelayFormatOpenAIResponses, types.RelayFormatGemini)
	require.True(t, ok, "responses→gemini 流式组合链应已注册")

	var chunks []*dto.GeminiChatResponse
	err := fn(context.Background(), nil, strings.NewReader(sse), func(chunk any) error {
		if g, ok := chunk.(*dto.GeminiChatResponse); ok {
			chunks = append(chunks, g)
		}
		return nil
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(chunks), 2, "文本 chunk + 尾 chunk")
	um := chunks[len(chunks)-1].UsageMetadata
	if assert.NotNil(t, um) {
		assert.Equal(t, 5, um.TotalTokenCount)
	}
	_ = io.Discard
}
