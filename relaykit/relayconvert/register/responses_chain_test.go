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

// TestResponsesToClaudeChain_ParallelToolResults 回归：codex 对 Claude 单轮并行多工具
// （assistant 一轮多个 function_call）各回一条 function_call_output——Claude 要求全部
// tool_result 位于紧随的下一条 user 消息，逐条映射为连续多条 user/tool_result 消息会被
// 上游 400 拒绝。链式转换必须聚合为单条 user 消息（多个 tool_result 块）。
func TestResponsesToClaudeChain_ParallelToolResults(t *testing.T) {
	spec, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterOpenAIResponsesToClaudeMessages)
	require.True(t, ok, "chain converter not registered")

	req := &dto.OpenAIResponsesRequest{
		Model: "claude-sonnet-4-5",
		Input: json.RawMessage(`[
			{"type": "message", "role": "user", "content": "看看两个文件"},
			{"type": "function_call", "call_id": "call_A", "name": "shell", "arguments": "{\"command\":[\"cat a\"]}"},
			{"type": "function_call", "call_id": "call_B", "name": "shell", "arguments": "{\"command\":[\"cat b\"]}"},
			{"type": "function_call_output", "call_id": "call_A", "output": "content of a"},
			{"type": "function_call_output", "call_id": "call_B", "output": "content of b"}
		]`),
		Tools:           json.RawMessage(`[{"type": "function", "name": "shell", "parameters": {"type": "object"}}]`),
		MaxOutputTokens: uintPtr(4096),
	}
	info := &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "claude-sonnet-4-5",
		UpstreamModelName:   "claude-sonnet-4-5-20251001",
	}

	result, err := relayconvert.ExecuteRequestConverter(context.Background(), spec, info, req)
	require.NoError(t, err)
	claudeReq, ok := result.(*dto.ClaudeRequest)
	require.True(t, ok, "chain result type = %T, want *dto.ClaudeRequest", result)

	// user → assistant(2 tool_use) → user(2 tool_result 合一)
	require.Len(t, claudeReq.Messages, 3, "messages: %+v", claudeReq.Messages)
	merged := claudeReq.Messages[2]
	require.Equal(t, "user", merged.Role)
	blocks, ok := merged.Content.([]dto.ClaudeContentBlock)
	require.True(t, ok, "merged content type = %T", merged.Content)
	require.Len(t, blocks, 2)
	require.Equal(t, "tool_result", blocks[0].Type)
	require.Equal(t, "call_A", blocks[0].ToolUseID)
	require.Equal(t, "tool_result", blocks[1].Type)
	require.Equal(t, "call_B", blocks[1].ToolUseID)
}

func uintPtr(v uint) *uint { return &v }
