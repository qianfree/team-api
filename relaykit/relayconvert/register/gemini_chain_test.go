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

// TestGeminiToClaudeChain gemini→openai→claude 链式 spec 经 ExecuteRequestConverter
// 逐跳执行的集成验证（R6 类型契约：第一跳必须输出 *dto.GeneralOpenAIRequest 供第二跳断言）。
// 用例覆盖 g2o 的 ID 合成/重排与 o2c 第二跳的组合行为。
func TestGeminiToClaudeChain(t *testing.T) {
	spec, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterGeminiContentToClaudeMessages)
	require.True(t, ok, "chain converter not registered")
	require.Empty(t, spec.Convert, "chain spec must not declare direct Convert")
	require.Len(t, spec.StepConverters, 2, "chain must have two steps")
	assert.Equal(t, relayconvert.ConverterGeminiContentToOpenAIChat, spec.StepConverters[0])
	assert.Equal(t, relayconvert.ConverterOpenAIChatToClaudeMessages, spec.StepConverters[1])

	// 含 functionCall/functionResponse 历史与 thinking 配置的 gemini 请求
	geminiReq := &dto.GeminiChatRequest{
		Contents: []dto.GeminiContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "跑测试"}}},
			{Role: "model", Parts: []dto.GeminiPart{
				{FunctionCall: &dto.GeminiFunctionCall{FunctionName: "run_tests", Arguments: map[string]any{"file": "a.go"}}},
			}},
			{Role: "user", Parts: []dto.GeminiPart{
				{FunctionResponse: &dto.GeminiFunctionResponse{Name: "run_tests", Response: map[string]any{"ok": true}}},
			}},
			{Role: "user", Parts: []dto.GeminiPart{{Text: "都通过了，帮我提交"}}},
		},
		GenerationConfig: &dto.GeminiGenerationConfig{MaxOutputTokens: new(uint), ThinkingConfig: nil},
	}
	*geminiReq.GenerationConfig.MaxOutputTokens = 1024
	info := &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "claude-sonnet-4",
		UpstreamModelName:   "claude-sonnet-4-20250514",
		Options: &convmeta.Options{
			Claude: convmeta.ClaudeOptions{DefaultMaxTokens: func(string) int { return 8192 }},
		},
	}

	result, err := relayconvert.ExecuteRequestConverter(context.Background(), spec, info, geminiReq)
	require.NoError(t, err, "chain execution failed（疑似 R6 类型契约断裂：第一跳输出类型与第二跳断言不符）")

	claudeReq, ok := result.(*dto.ClaudeRequest)
	require.True(t, ok, "chain result type = %T, want *dto.ClaudeRequest", result)

	assert.Equal(t, "claude-sonnet-4-20250514", claudeReq.Model)
	if assert.NotNil(t, claudeReq.MaxTokens) {
		assert.EqualValues(t, 1024, *claudeReq.MaxTokens, "maxOutputTokens 应透传为 max_tokens")
	}
	// 消息序列：user / assistant(tool_use) / user(tool_result 块) / user —— 第二跳按 Claude
	// 协议把 chat 的 tool 消息包装进 user 消息的 tool_result 块（协议正确形态）
	require.Len(t, claudeReq.Messages, 4, "messages 应为 user+assistant+user(tool_result)+user")
	assert.Equal(t, "user", claudeReq.Messages[0].Role)
	assert.Equal(t, "assistant", claudeReq.Messages[1].Role)
	assert.Equal(t, "user", claudeReq.Messages[2].Role, "tool 结果应包装为 user 消息内的 tool_result 块")
	assert.Equal(t, "user", claudeReq.Messages[3].Role)
	// tool_result 块通过 JSON 内容携带（o2c 的 user 消息内容形态），验证工具调用链路无损
	msg2JSON, _ := json.Marshal(claudeReq.Messages[2].Content)
	assert.Contains(t, string(msg2JSON), "tool_result", "user 消息应含 tool_result 块")
	assert.Contains(t, string(msg2JSON), "call_0", "tool_result 应引用合成的 call_0 ID")
}

// TestClaudeAndGeminiToOpenAIChatRegistered spec A/B 的注册与执行面验证。
func TestClaudeAndGeminiToOpenAIChatRegistered(t *testing.T) {
	// spec A：claude→openai
	specA, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterClaudeMessagesToOpenAIChat)
	require.True(t, ok, "spec A not registered")
	require.NotNil(t, specA.Convert)

	maxTokens := uint(100)
	claudeReq := &dto.ClaudeRequest{
		Model:     "ignored",
		MaxTokens: &maxTokens,
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
	}
	resA, err := relayconvert.ExecuteRequestConverter(context.Background(), specA, nil, claudeReq)
	require.NoError(t, err)
	chatA, ok := resA.(*dto.GeneralOpenAIRequest)
	require.True(t, ok, "spec A result = %T", resA)
	assert.Equal(t, "hi", chatA.Messages[0].Content)

	// spec B：gemini→openai
	specB, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterGeminiContentToOpenAIChat)
	require.True(t, ok, "spec B not registered")
	require.NotNil(t, specB.Convert)

	geminiReq := &dto.GeminiChatRequest{
		Contents: []dto.GeminiContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}}},
	}
	resB, err := relayconvert.ExecuteRequestConverter(context.Background(), specB, nil, geminiReq)
	require.NoError(t, err)
	chatB, ok := resB.(*dto.GeneralOpenAIRequest)
	require.True(t, ok, "spec B result = %T", resB)
	assert.Equal(t, "hi", chatB.Messages[0].Content)
}
