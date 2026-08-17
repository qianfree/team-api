package register

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResponsesToClaudeChain 验证 Responses→Claude 链式 spec 经 ExecuteRequestConverter
// 逐跳执行：instructions 落到 Claude 的 system、input 项转为 Claude 消息、
// 工具映射到 Claude tools。
func TestResponsesToClaudeChain(t *testing.T) {
	spec, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterOpenAIResponsesToClaudeMessages)
	require.True(t, ok, "chain converter not registered")
	require.Empty(t, spec.Convert, "chain spec must not declare direct Convert")
	require.Len(t, spec.StepConverters, 2, "chain must have two steps")
	assert.Equal(t, relayconvert.ConverterOpenAIResponsesToOpenAIChat, spec.StepConverters[0])
	assert.Equal(t, relayconvert.ConverterOpenAIChatToClaudeMessages, spec.StepConverters[1])

	req := &dto.OpenAIResponsesRequest{
		Model:        "claude-sonnet-4",
		Instructions: json.RawMessage(`"你是编码助手"`),
		Input: json.RawMessage(`[
			{"type": "message", "role": "user", "content": "写一个加法函数"},
			{"type": "function_call", "call_id": "call_1", "name": "run", "arguments": "{\"a\":1}"},
			{"type": "function_call_output", "call_id": "call_1", "output": "PASS"}
		]`),
		MaxOutputTokens: uintPtr(1024),
		Tools: json.RawMessage(`[{"type": "function", "name": "run", "description": "执行",
			"parameters": {"type": "object", "properties": {"a": {"type": "integer"}}}}]`),
	}
	info := &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "claude-sonnet-4",
		UpstreamModelName:   "claude-sonnet-4-20250514",
		Options: &convmeta.Options{
			Claude: convmeta.ClaudeOptions{DefaultMaxTokens: func(string) int { return 8192 }},
		},
	}

	result, err := relayconvert.ExecuteRequestConverter(context.Background(), spec, info, req)
	require.NoError(t, err, "chain execution failed")

	claudeReq, ok := result.(*dto.ClaudeRequest)
	require.True(t, ok, "chain result type = %T, want *dto.ClaudeRequest", result)

	// 模型取上游映射名；instructions → system；user/tool 历史转为 Claude 消息
	assert.Equal(t, "claude-sonnet-4-20250514", claudeReq.Model)
	if assert.NotNil(t, claudeReq.System) {
		assert.Contains(t, systemToString(t, claudeReq.System), "你是编码助手")
	}
	assert.NotEmpty(t, claudeReq.Messages, "messages should contain user + tool history")
	if assert.Len(t, claudeReq.Tools, 1) {
		assert.Equal(t, "run", claudeReq.Tools[0].Name)
	}
	if claudeReq.MaxTokens != nil {
		assert.EqualValues(t, 1024, *claudeReq.MaxTokens, "max_output_tokens 应透传为 max_tokens")
	}
}

// systemToString 提取 Claude system 字段的文本（字符串或内容块数组两种形态）。
func systemToString(t *testing.T, system any) string {
	t.Helper()
	switch v := system.(type) {
	case string:
		return v
	default:
		b, err := json.Marshal(v)
		require.NoError(t, err)
		return string(b)
	}
}

func uintPtr(v uint) *uint { return &v }
